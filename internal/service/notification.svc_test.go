package service

import (
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"gorm.io/gorm"
)

type notificationFixture struct {
	notifications *NotificationSvc
	chat          *ChatSvc
	rooms         *RoomSvc
	owner         Principal
	guest         Principal
	third         Principal
	requester     Principal
	room          RoomSnapshot
	pending       JoinRequestView
	db            *gorm.DB
	hub           *StreamHub
}

func newNotificationFixture(t *testing.T) notificationFixture {
	t.Helper()
	sessions, db, _ := newSessionTestService(t)
	createPrincipal := func(name string) Principal {
		t.Helper()
		result, err := sessions.Create(name, "")
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return result.Principal
	}
	owner := createPrincipal("甲一")
	guest := createPrincipal("乙二")
	third := createPrincipal("丙三")
	requester := createPrincipal("丁四")
	hub := NewStreamHub()
	rooms, err := NewRoomSvc(db, hub)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	room, err := rooms.Create(owner.UserID, model.JoinModeOwnerApproval, "", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	joinAndApprove := func(principal Principal) {
		t.Helper()
		request, err := rooms.CreateJoinRequest(principal.UserID, room.RoomCode)
		if err != nil {
			t.Fatalf("create join request for %s: %v", principal.DisplayName, err)
		}
		if err := rooms.ApproveJoinRequest(owner.UserID, room.RoomCode, request.RequestID); err != nil {
			t.Fatalf("approve %s: %v", principal.DisplayName, err)
		}
	}
	joinAndApprove(guest)
	joinAndApprove(third)
	pending, err := rooms.CreateJoinRequest(requester.UserID, room.RoomCode)
	if err != nil {
		t.Fatalf("create pending request: %v", err)
	}
	chat, err := NewChatSvc(db, hub, NewRateLimiter(), nil)
	if err != nil {
		t.Fatalf("new chat service: %v", err)
	}
	notifications, err := NewNotificationSvc(db, hub)
	if err != nil {
		t.Fatalf("new notification service: %v", err)
	}
	return notificationFixture{notifications: notifications, chat: chat, rooms: rooms, owner: owner, guest: guest, third: third, requester: requester, room: room, pending: pending, db: db, hub: hub}
}

func TestNotificationAggregatesApprovalsAndUnreadChats(t *testing.T) {
	fixture := newNotificationFixture(t)
	base := time.Now().Add(-time.Minute).Truncate(time.Second)
	fixture.chat.now = func() time.Time { return base }
	first, err := fixture.chat.Send(fixture.guest.UserID, fixture.room.RoomCode, fixture.owner.UserID, "guest-1", "第一条")
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	fixture.chat.now = func() time.Time { return base.Add(time.Second) }
	second, err := fixture.chat.Send(fixture.guest.UserID, fixture.room.RoomCode, fixture.owner.UserID, "guest-2", "第二条")
	if err != nil {
		t.Fatalf("send second: %v", err)
	}
	fixture.chat.now = func() time.Time { return base.Add(2 * time.Second) }
	third, err := fixture.chat.Send(fixture.third.UserID, fixture.room.RoomCode, fixture.owner.UserID, "third-1", "第三人的消息")
	if err != nil {
		t.Fatalf("send third: %v", err)
	}

	page, err := fixture.notifications.List(fixture.owner.UserID, "", 1)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if page.TotalCount != 4 || len(page.Items) != 1 || page.Items[0].Type != NotificationTypeJoinRequest || page.NextCursor == "" {
		t.Fatalf("first page = %+v", page)
	}
	secondPage, err := fixture.notifications.List(fixture.owner.UserID, page.NextCursor, 30)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if secondPage.TotalCount != 4 || len(secondPage.Items) != 2 {
		t.Fatalf("second page = %+v", secondPage)
	}
	guestItem := findChatNotification(t, secondPage.Items, fixture.guest.UserID)
	if guestItem.UnreadCount != 2 || guestItem.LatestMessageText != "第二条" || guestItem.LatestMessageAt != second.CreatedAt {
		t.Fatalf("guest notification = %+v", guestItem)
	}
	thirdItem := findChatNotification(t, secondPage.Items, fixture.third.UserID)
	if thirdItem.UnreadCount != 1 || thirdItem.LatestMessageText != "第三人的消息" || thirdItem.LatestMessageAt != third.CreatedAt {
		t.Fatalf("third notification = %+v", thirdItem)
	}

	if _, err := fixture.chat.MarkRead(fixture.owner.UserID, fixture.room.RoomCode, fixture.guest.UserID, first.Sequence); err != nil {
		t.Fatalf("mark first read: %v", err)
	}
	if _, err := fixture.chat.Recall(fixture.guest.UserID, fixture.room.RoomCode, second.MessageID); err != nil {
		t.Fatalf("recall second: %v", err)
	}
	page, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list after recall: %v", err)
	}
	if page.TotalCount != 2 || len(page.Items) != 2 {
		t.Fatalf("after recall page = %+v", page)
	}
	if err := fixture.chat.DeleteMessage(fixture.owner.UserID, fixture.room.RoomCode, third.MessageID); err != nil {
		t.Fatalf("delete third locally: %v", err)
	}
	if err := fixture.rooms.ApproveJoinRequest(fixture.owner.UserID, fixture.room.RoomCode, fixture.pending.RequestID); err != nil {
		t.Fatalf("approve pending: %v", err)
	}
	page, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if page.TotalCount != 0 || len(page.Items) != 0 {
		t.Fatalf("final page = %+v", page)
	}
}

func TestNotificationKeepsRoomsSeparateAndFiltersInvalidMembers(t *testing.T) {
	fixture := newNotificationFixture(t)
	if _, err := fixture.chat.Send(fixture.guest.UserID, fixture.room.RoomCode, fixture.owner.UserID, "room-one", "第一个房间"); err != nil {
		t.Fatalf("send in first room: %v", err)
	}
	secondRoom, err := fixture.rooms.Create(fixture.guest.UserID, model.JoinModeOpen, "", "")
	if err != nil {
		t.Fatalf("create second room: %v", err)
	}
	if _, err := fixture.rooms.Join(fixture.owner.UserID, secondRoom.RoomCode, true, ""); err != nil {
		t.Fatalf("join second room: %v", err)
	}
	if _, err := fixture.chat.Send(fixture.guest.UserID, secondRoom.RoomCode, fixture.owner.UserID, "room-two", "第二个房间"); err != nil {
		t.Fatalf("send in second room: %v", err)
	}
	page, err := fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list cross-room notifications: %v", err)
	}
	chatItems := 0
	for _, item := range page.Items {
		if item.Type == NotificationTypeChatConversation {
			chatItems++
		}
	}
	if chatItems != 2 || page.TotalCount != 3 {
		t.Fatalf("cross-room page = %+v", page)
	}
	if err := fixture.rooms.Leave(fixture.owner.UserID, secondRoom.RoomCode); err != nil {
		t.Fatalf("leave second room: %v", err)
	}
	if err := fixture.rooms.Leave(fixture.guest.UserID, fixture.room.RoomCode); err != nil {
		t.Fatalf("peer leave first room: %v", err)
	}
	page, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list after members invalid: %v", err)
	}
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].Type != NotificationTypeJoinRequest {
		t.Fatalf("invalid members were not filtered: %+v", page)
	}
}

func TestNotificationLocalDeletionOnlyAffectsDeletingUser(t *testing.T) {
	fixture := newNotificationFixture(t)
	message, err := fixture.chat.Send(fixture.guest.UserID, fixture.room.RoomCode, fixture.owner.UserID, "delete-scope", "只对自己删除")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := fixture.chat.DeleteMessage(fixture.guest.UserID, fixture.room.RoomCode, message.MessageID); err != nil {
		t.Fatalf("sender local delete: %v", err)
	}
	page, err := fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list after sender delete: %v", err)
	}
	if page.TotalCount != 2 {
		t.Fatalf("sender delete changed recipient notification: %+v", page)
	}
	if err := fixture.chat.DeleteMessage(fixture.owner.UserID, fixture.room.RoomCode, message.MessageID); err != nil {
		t.Fatalf("recipient local delete: %v", err)
	}
	page, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatalf("list after recipient delete: %v", err)
	}
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].Type != NotificationTypeJoinRequest {
		t.Fatalf("recipient deletion did not clear chat notification: %+v", page)
	}
}

func TestNotificationFiltersInvalidRoomAndRequestLifecycle(t *testing.T) {
	fixture := newNotificationFixture(t)
	if _, err := fixture.chat.Send(fixture.guest.UserID, fixture.room.RoomCode, fixture.owner.UserID, "lifecycle", "生命周期消息"); err != nil {
		t.Fatalf("send: %v", err)
	}
	assertTotal := func(want int64) {
		t.Helper()
		page, err := fixture.notifications.List(fixture.owner.UserID, "", 30)
		if err != nil {
			t.Fatalf("list notifications: %v", err)
		}
		if page.TotalCount != want {
			t.Fatalf("total count = %d, want %d; page=%+v", page.TotalCount, want, page)
		}
	}
	assertTotal(2)

	if err := fixture.db.Model(&model.Room{}).Where("id = ?", fixture.room.RoomID).Update("status", model.RoomStatusDestroying).Error; err != nil {
		t.Fatalf("mark room destroying: %v", err)
	}
	assertTotal(0)

	if err := fixture.db.Model(&model.Room{}).Where("id = ?", fixture.room.RoomID).Updates(map[string]interface{}{
		"status":     model.RoomStatusActive,
		"expires_at": fixture.notifications.now().Add(-time.Second).Unix(),
	}).Error; err != nil {
		t.Fatalf("expire room: %v", err)
	}
	assertTotal(0)

	if err := fixture.db.Model(&model.Room{}).Where("id = ?", fixture.room.RoomID).Updates(map[string]interface{}{
		"expires_at": fixture.notifications.now().Add(time.Hour).Unix(),
		"join_mode":  model.JoinModeOpen,
	}).Error; err != nil {
		t.Fatalf("change room join mode: %v", err)
	}
	assertTotal(1)

	if err := fixture.db.Model(&model.Room{}).Where("id = ?", fixture.room.RoomID).Update("join_mode", model.JoinModeOwnerApproval).Error; err != nil {
		t.Fatalf("restore room join mode: %v", err)
	}
	if err := fixture.db.Model(&model.JoinRequest{}).Where("id = ?", fixture.pending.RequestID).Update("expires_at", fixture.notifications.now().Add(-time.Second).Unix()).Error; err != nil {
		t.Fatalf("expire join request: %v", err)
	}
	assertTotal(1)
}

func TestNotificationRejectsInvalidPagination(t *testing.T) {
	fixture := newNotificationFixture(t)
	for _, limit := range []int{-1, NotificationMaxPageSize + 1} {
		if _, err := fixture.notifications.List(fixture.owner.UserID, "", limit); err == nil || errx.Code(err) != errc.ErrParams {
			t.Fatalf("limit %d error = %v, code=%d", limit, err, errx.Code(err))
		}
	}
	if _, err := fixture.notifications.List(fixture.owner.UserID, "not-a-cursor", 30); err == nil || errx.Code(err) != errc.ErrParams {
		t.Fatalf("cursor error = %v, code=%d", err, errx.Code(err))
	}
}

func TestNotificationAggregatesPendingPrivateFiles(t *testing.T) {
	fixture := newNotificationFixture(t)
	baseMS := time.Now().Add(-time.Minute).UnixMilli()
	first := createFileNotificationFixture(t, fixture, "received-one", NotificationTypeFileReceived, fixture.guest.UserID, fixture.owner.UserID, model.RecipientPending, "第一份.txt", baseMS)
	createFileNotificationFixture(t, fixture, "received-two", NotificationTypeFileReceived, fixture.guest.UserID, fixture.owner.UserID, model.RecipientPending, "第二份.txt", baseMS+1000)

	page, err := fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 2 || len(page.Items) != 1 {
		t.Fatalf("pending file page = %+v", page)
	}
	item := page.Items[0]
	if item.Type != NotificationTypeFileReceived || item.FileCount != 2 || item.LatestFileName != "第二份.txt" || item.CounterpartUserID != fixture.owner.UserID || item.ReadToken != "" {
		t.Fatalf("pending file item = %+v", item)
	}
	if err := fixture.db.Model(&model.FileRecipient{}).Where("id = ?", first.FileRecipientID).Update("status", model.RecipientAccepted).Error; err != nil {
		t.Fatal(err)
	}
	page, err = fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil || page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].FileCount != 1 {
		t.Fatalf("pending file after accept = %+v, %v", page, err)
	}
}

func TestNotificationMarksOnlyFileResultsThroughSnapshot(t *testing.T) {
	fixture := newNotificationFixture(t)
	baseMS := time.Now().Add(-time.Minute).UnixMilli()
	createFileNotificationFixture(t, fixture, "declined-one", NotificationTypeFileDeclined, fixture.owner.UserID, fixture.guest.UserID, model.RecipientDeclined, "拒绝一.txt", baseMS)
	createFileNotificationFixture(t, fixture, "declined-two", NotificationTypeFileDeclined, fixture.owner.UserID, fixture.guest.UserID, model.RecipientDeclined, "拒绝二.txt", baseMS+1000)
	createFileNotificationFixture(t, fixture, "downloaded-one", NotificationTypeFileDownloaded, fixture.owner.UserID, fixture.guest.UserID, model.RecipientDownloaded, "完成一.txt", baseMS+2000)

	page, err := fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 4 || len(page.Items) != 3 || page.Items[0].Type != NotificationTypeJoinRequest {
		t.Fatalf("file result page = %+v", page)
	}
	declined := findFileNotification(t, page.Items, NotificationTypeFileDeclined)
	if declined.FileCount != 2 || declined.ReadToken == "" || declined.LatestFileName != "拒绝二.txt" {
		t.Fatalf("declined group = %+v", declined)
	}
	createFileNotificationFixture(t, fixture, "declined-three", NotificationTypeFileDeclined, fixture.owner.UserID, fixture.guest.UserID, model.RecipientDeclined, "拒绝三.txt", baseMS+3000)

	events, stop := fixture.hub.Subscribe(fixture.owner.UserID)
	defer stop()
	updated, err := fixture.notifications.MarkFileResultsRead(fixture.owner.UserID, declined.Key, declined.ReadToken)
	if err != nil || updated != 2 {
		t.Fatalf("mark file results = %d, %v", updated, err)
	}
	updated, err = fixture.notifications.MarkFileResultsRead(fixture.owner.UserID, declined.Key, declined.ReadToken)
	if err != nil || updated != 0 {
		t.Fatalf("repeat mark file results = %d, %v", updated, err)
	}
	foundChanged := false
	for len(events) > 0 {
		if event := <-events; event.Type == "notification.changed" {
			foundChanged = true
		}
	}
	if !foundChanged {
		t.Fatal("notification.changed was not published")
	}

	page, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	remainingDeclined := findFileNotification(t, page.Items, NotificationTypeFileDeclined)
	if page.TotalCount != 3 || remainingDeclined.FileCount != 1 || remainingDeclined.LatestFileName != "拒绝三.txt" {
		t.Fatalf("remaining file result page = %+v", page)
	}
	if _, err := fixture.notifications.MarkFileResultsRead(fixture.owner.UserID, remainingDeclined.Key, "invalid"); errx.Code(err) != errc.ErrParams {
		t.Fatalf("invalid read token error = %v", err)
	}
}

func createFileNotificationFixture(t *testing.T, fixture notificationFixture, suffix, notificationType, notificationUserID, counterpartUserID, recipientStatus, fileName string, occurredAtMS int64) model.NotificationRecord {
	t.Helper()
	fileID := "file-" + suffix
	relationID := "relation-" + suffix
	file := model.RoomFile{ID: fileID, RoomID: fixture.room.RoomID, BatchID: "batch-" + suffix, UploaderUserID: fixture.owner.UserID, StorageName: "storage-" + suffix, OriginalName: fileName, DeclaredSize: 1, ActualSize: 1, Scope: model.FileScopeDirect, Status: model.FileStatusAvailable, Progress: 100, CreatedAt: occurredAtMS / 1000}
	if err := fixture.db.Create(&file).Error; err != nil {
		t.Fatalf("create notification file: %v", err)
	}
	recipient := model.FileRecipient{ID: relationID, FileID: fileID, RecipientUserID: fixture.guest.UserID, DeliveryVersion: 1, Status: recipientStatus, SentAt: occurredAtMS / 1000}
	if recipientStatus == model.RecipientDeclined {
		value := occurredAtMS / 1000
		recipient.DeclinedAt = &value
	}
	if recipientStatus == model.RecipientDownloaded {
		value := occurredAtMS / 1000
		recipient.FirstDownloadedAt = &value
		recipient.LastDownloadedAt = &value
		recipient.DownloadCount = 1
	}
	if err := fixture.db.Create(&recipient).Error; err != nil {
		t.Fatalf("create notification recipient: %v", err)
	}
	record := model.NotificationRecord{ID: "record-" + suffix, UserID: notificationUserID, Type: notificationType, OccurrenceKey: "recipient:" + relationID + ":1", RoomID: fixture.room.RoomID, FileID: fileID, CounterpartUserID: counterpartUserID, FileRecipientID: relationID, DeliveryVersion: 1, SourceEventID: "event-" + suffix, OccurredAtMS: occurredAtMS}
	if err := fixture.db.Create(&record).Error; err != nil {
		t.Fatalf("create notification record: %v", err)
	}
	return record
}

func findFileNotification(t *testing.T, items []NotificationItem, notificationType string) NotificationItem {
	t.Helper()
	for _, item := range items {
		if item.Type == notificationType {
			return item
		}
	}
	t.Fatalf("file notification %s not found in %+v", notificationType, items)
	return NotificationItem{}
}

func findChatNotification(t *testing.T, items []NotificationItem, peerUserID string) NotificationItem {
	t.Helper()
	for _, item := range items {
		if item.Type == NotificationTypeChatConversation && item.PeerUserID == peerUserID {
			return item
		}
	}
	t.Fatalf("chat notification for %s not found in %+v", peerUserID, items)
	return NotificationItem{}
}
