package service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

func TestTrashResultNotificationsAggregateAndUseReadWatermark(t *testing.T) {
	fixture := newNotificationFixture(t)
	files := newNotificationFileService(t, fixture)
	base := time.Now().Add(-time.Minute).Truncate(time.Second)

	first := createNotificationTestFile(t, files, fixture, "trash-notify-one", model.FileScopeShared, "第一份.txt", nil)
	files.now = func() time.Time { return base }
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, first.ID, "first reason"); err != nil {
		t.Fatal(err)
	}
	second := createNotificationTestFile(t, files, fixture, "trash-notify-two", model.FileScopeShared, "第二份.txt", nil)
	files.now = func() time.Time { return base.Add(time.Second) }
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, second.ID, "second reason"); err != nil {
		t.Fatal(err)
	}

	page, err := fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	item := findFileNotification(t, page.Items, NotificationTypeFileTrashedByOwner)
	if page.TotalCount != 2 || item.FileCount != 2 || item.LatestFileName != "第二份.txt" || item.LatestReason != "second reason" || item.ReadToken == "" {
		t.Fatalf("trash notification page=%+v item=%+v", page, item)
	}
	updated, err := fixture.notifications.MarkFileResultsRead(fixture.guest.UserID, item.Key, item.ReadToken)
	if err != nil || updated != 2 {
		t.Fatalf("mark trash notifications=%d error=%v", updated, err)
	}

	third := createNotificationTestFile(t, files, fixture, "trash-notify-three", model.FileScopeShared, "第三份.txt", nil)
	files.now = func() time.Time { return base.Add(2 * time.Second) }
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, third.ID, "third reason"); err != nil {
		t.Fatal(err)
	}
	page, err = fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	item = findFileNotification(t, page.Items, NotificationTypeFileTrashedByOwner)
	if page.TotalCount != 1 || item.FileCount != 1 || item.LatestFileName != "第三份.txt" || item.LatestReason != "third reason" {
		t.Fatalf("watermark page=%+v", page)
	}
}

func TestRestoreRequestNotificationIsActionablePrivateAndInvalidatesWithMembership(t *testing.T) {
	fixture := newNotificationFixture(t)
	files := newNotificationFileService(t, fixture)
	fixture.rooms.AttachFileService(files)
	file := createNotificationTestFile(t, files, fixture, "restore-private", model.FileScopeDirect, "不能泄露的文件.txt", []string{fixture.third.UserID})
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, file.ID, "capacity cleanup"); err != nil {
		t.Fatal(err)
	}
	action, err := files.RestoreFile(fixture.guest.UserID, fixture.room.RoomCode, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	ownerPage, err := fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	requestItem := findFileNotification(t, ownerPage.Items, NotificationTypeFileRestoreRequested)
	if requestItem.RequestID != action.RequestID || requestItem.LatestFileName != file.PrivateCode || requestItem.LatestFileName == file.OriginalName || requestItem.LatestReason != "capacity cleanup" || requestItem.ReadToken != "" || requestItem.FileCount != 1 {
		t.Fatalf("restore request item=%+v", requestItem)
	}
	if _, err := fixture.notifications.MarkFileResultsRead(fixture.owner.UserID, requestItem.Key, requestItem.ReadToken); errx.Code(err) != errc.ErrParams {
		t.Fatalf("action notification was markable: %v", err)
	}

	if err := fixture.rooms.Leave(fixture.guest.UserID, fixture.room.RoomCode); err != nil {
		t.Fatal(err)
	}
	ownerPage, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil || hasNotificationType(ownerPage.Items, NotificationTypeFileRestoreRequested) {
		t.Fatalf("invalidated request page=%+v error=%v", ownerPage, err)
	}
	rejoin, err := fixture.rooms.CreateJoinRequest(fixture.guest.UserID, fixture.room.RoomCode)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.rooms.ApproveJoinRequest(fixture.owner.UserID, fixture.room.RoomCode, rejoin.RequestID); err != nil {
		t.Fatal(err)
	}
	reactivated, err := files.RestoreFile(fixture.guest.UserID, fixture.room.RoomCode, file.ID)
	if err != nil || reactivated.RequestID != action.RequestID {
		t.Fatalf("reactivated=%+v error=%v", reactivated, err)
	}
	ownerPage, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil || !hasNotificationType(ownerPage.Items, NotificationTypeFileRestoreRequested) {
		t.Fatalf("reactivated request page=%+v error=%v", ownerPage, err)
	}
	if err := files.RejectFileRestore(fixture.owner.UserID, fixture.room.RoomCode, action.RequestID, "not needed"); err != nil {
		t.Fatal(err)
	}
	ownerPage, err = fixture.notifications.List(fixture.owner.UserID, "", 30)
	if err != nil || hasNotificationType(ownerPage.Items, NotificationTypeFileRestoreRequested) {
		t.Fatalf("rejected owner page=%+v error=%v", ownerPage, err)
	}
	guestPage, err := fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	rejected := findFileNotification(t, guestPage.Items, NotificationTypeFileRestoreRejected)
	if rejected.LatestReason != "not needed" || rejected.LatestFileName != file.OriginalName || rejected.ReadToken == "" {
		t.Fatalf("rejected result=%+v", rejected)
	}
}

func TestOwnerRestoreAndPurgeNotificationsReplaceObsoleteLifecycleResults(t *testing.T) {
	fixture := newNotificationFixture(t)
	files := newNotificationFileService(t, fixture)

	restoredFile := createNotificationTestFile(t, files, fixture, "restore-result", model.FileScopeShared, "恢复结果.txt", nil)
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, restoredFile.ID, "mistake"); err != nil {
		t.Fatal(err)
	}
	request, err := files.RestoreFile(fixture.guest.UserID, fixture.room.RoomCode, restoredFile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.ApproveFileRestore(fixture.owner.UserID, fixture.room.RoomCode, request.RequestID); err != nil {
		t.Fatal(err)
	}
	page, err := fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if hasNotificationType(page.Items, NotificationTypeFileTrashedByOwner) || hasNotificationType(page.Items, NotificationTypeFileRestoreRejected) {
		t.Fatalf("obsolete restore notifications remained: %+v", page)
	}
	restored := findFileNotification(t, page.Items, NotificationTypeFileRestoredByOwner)
	if restored.LatestFileName != restoredFile.OriginalName || restored.LatestReason != "mistake" {
		t.Fatalf("restored notification=%+v", restored)
	}
	if _, err := fixture.notifications.MarkFileResultsRead(fixture.guest.UserID, restored.Key, restored.ReadToken); err != nil {
		t.Fatal(err)
	}

	purgedFile := createNotificationTestFile(t, files, fixture, "purge-result", model.FileScopeShared, "永久删除.txt", nil)
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, purgedFile.ID, "policy cleanup"); err != nil {
		t.Fatal(err)
	}
	if err := files.PurgeFile(fixture.owner.UserID, fixture.room.RoomCode, purgedFile.ID); err != nil {
		t.Fatal(err)
	}
	page, err = fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if hasNotificationType(page.Items, NotificationTypeFileTrashedByOwner) {
		t.Fatalf("trash notification remained after purge: %+v", page)
	}
	purged := findFileNotification(t, page.Items, NotificationTypeFilePurgedByOwner)
	if page.TotalCount != 1 || purged.LatestFileName != purgedFile.OriginalName || purged.LatestReason != "policy cleanup" {
		t.Fatalf("purged page=%+v item=%+v", page, purged)
	}
}

func TestTrashPermanentlyInvalidatesOldResultsButPendingDeliveryReturns(t *testing.T) {
	fixture := newNotificationFixture(t)
	files := newNotificationFileService(t, fixture)

	declinedFile := createNotificationTestFile(t, files, fixture, "old-declined", model.FileScopeDirect, "已拒绝.txt", []string{fixture.third.UserID})
	if err := files.DeclineFile(fixture.third.UserID, fixture.room.RoomCode, declinedFile.ID); err != nil {
		t.Fatal(err)
	}
	guestPage, err := fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil || !hasNotificationType(guestPage.Items, NotificationTypeFileDeclined) {
		t.Fatalf("missing declined notification page=%+v error=%v", guestPage, err)
	}
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, declinedFile.ID, "cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := files.RestoreFile(fixture.owner.UserID, fixture.room.RoomCode, declinedFile.ID); err != nil {
		t.Fatal(err)
	}
	guestPage, err = fixture.notifications.List(fixture.guest.UserID, "", 30)
	if err != nil || hasNotificationType(guestPage.Items, NotificationTypeFileDeclined) {
		t.Fatalf("declined notification revived page=%+v error=%v", guestPage, err)
	}

	pendingFile := createNotificationTestFile(t, files, fixture, "pending-return", model.FileScopeDirect, "待接收.txt", []string{fixture.third.UserID})
	thirdPage, err := fixture.notifications.List(fixture.third.UserID, "", 30)
	if err != nil || !hasNotificationType(thirdPage.Items, NotificationTypeFileReceived) {
		t.Fatalf("missing pending delivery page=%+v error=%v", thirdPage, err)
	}
	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, pendingFile.ID, "cleanup"); err != nil {
		t.Fatal(err)
	}
	thirdPage, err = fixture.notifications.List(fixture.third.UserID, "", 30)
	if err != nil || hasNotificationType(thirdPage.Items, NotificationTypeFileReceived) {
		t.Fatalf("pending delivery remained in trash page=%+v error=%v", thirdPage, err)
	}
	if _, err := files.RestoreFile(fixture.owner.UserID, fixture.room.RoomCode, pendingFile.ID); err != nil {
		t.Fatal(err)
	}
	thirdPage, err = fixture.notifications.List(fixture.third.UserID, "", 30)
	if err != nil || !hasNotificationType(thirdPage.Items, NotificationTypeFileReceived) {
		t.Fatalf("pending delivery did not return page=%+v error=%v", thirdPage, err)
	}
}

func TestTrashLifecyclePublishesNotificationChanges(t *testing.T) {
	fixture := newNotificationFixture(t)
	files := newNotificationFileService(t, fixture)
	file := createNotificationTestFile(t, files, fixture, "notification-sse", model.FileScopeShared, "同步.txt", nil)
	guestEvents, stopGuest := fixture.hub.Subscribe(fixture.guest.UserID)
	defer stopGuest()
	ownerEvents, stopOwner := fixture.hub.Subscribe(fixture.owner.UserID)
	defer stopOwner()

	if _, err := files.TrashFile(fixture.owner.UserID, fixture.room.RoomCode, file.ID, "cleanup"); err != nil {
		t.Fatal(err)
	}
	if !drainHasStreamEvent(guestEvents, "notification.changed") {
		t.Fatal("guest did not receive trash notification change")
	}
	request, err := files.RestoreFile(fixture.guest.UserID, fixture.room.RoomCode, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !drainHasStreamEvent(ownerEvents, "notification.changed") {
		t.Fatal("owner did not receive restore request notification change")
	}
	if err := files.RejectFileRestore(fixture.owner.UserID, fixture.room.RoomCode, request.RequestID, "no"); err != nil {
		t.Fatal(err)
	}
	if !drainHasStreamEvent(guestEvents, "notification.changed") {
		t.Fatal("guest did not receive rejection notification change")
	}
}

func newNotificationFileService(t *testing.T, fixture notificationFixture) *FileSvc {
	t.Helper()
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileSvc(fixture.db, fixture.hub, storage)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func createNotificationTestFile(t *testing.T, files *FileSvc, fixture notificationFixture, key, scope, name string, recipients []string) model.RoomFile {
	t.Helper()
	content := []byte("notification file")
	batch, err := files.CreateUploadBatch(fixture.guest.UserID, fixture.room.RoomCode, key, scope, []FileManifest{{OriginalName: name, DeclaredSize: int64(len(content)), DeclaredMIME: "text/plain"}}, recipients)
	if err != nil {
		t.Fatal(err)
	}
	file := batch.Files[0]
	if err := files.UploadContent(context.Background(), fixture.guest.UserID, fixture.room.RoomCode, file.ID, int64(len(content)), bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.First(&file, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	return file
}

func hasNotificationType(items []NotificationItem, notificationType string) bool {
	for _, item := range items {
		if item.Type == notificationType {
			return true
		}
	}
	return false
}

func drainHasStreamEvent(events <-chan *StreamEvent, eventType string) bool {
	found := false
	for len(events) > 0 {
		if event := <-events; event.Type == eventType {
			found = true
		}
	}
	return found
}
