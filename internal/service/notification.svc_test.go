package service

import (
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
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
	notifications, err := NewNotificationSvc(db)
	if err != nil {
		t.Fatalf("new notification service: %v", err)
	}
	return notificationFixture{notifications: notifications, chat: chat, rooms: rooms, owner: owner, guest: guest, third: third, requester: requester, room: room, pending: pending}
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
