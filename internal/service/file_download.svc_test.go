package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

func makeStoredAvailableFile(t *testing.T, fixture fileTestFixture, storage *FileStorage, key, scope, name string, content []byte, recipients []string) model.RoomFile {
	t.Helper()
	fixture.svc.storage = storage
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, key, scope, []FileManifest{{OriginalName: name, DeclaredSize: int64(len(content)), DeclaredMIME: "application/octet-stream"}}, recipients)
	if err != nil {
		t.Fatal(err)
	}
	file := batch.Files[0]
	if err := fixture.svc.MarkUploading(fixture.uploader.UserID, file.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := storage.Write(context.Background(), file.RoomID, file.StorageName, file.DeclaredSize, bytes.NewReader(content), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.CompleteUpload(fixture.uploader.UserID, file.ID, stored.DetectedMIME, stored.Size); err != nil {
		t.Fatal(err)
	}
	fixture.svc.db.First(&file, "id = ?", file.ID)
	return file
}

func TestDownloadTaskStreamsCompletesAndSupportsShortReplay(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("download content")
	file := makeStoredAvailableFile(t, fixture, storage, "download-shared", model.FileScopeShared, "报告.txt", content, nil)
	task, err := fixture.svc.CreateDownloadTask(fixture.outsider.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := fixture.svc.BeginDownload(fixture.outsider.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := stream.WriteTo(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), content) || stream.FileName() != "报告.txt" || stream.Size() != int64(len(content)) {
		t.Fatalf("download output=%q name=%q size=%d", output.Bytes(), stream.FileName(), stream.Size())
	}
	var storedTask model.DownloadTask
	fixture.svc.db.First(&storedTask, "id = ?", task.TaskID)
	if storedTask.Status != model.DownloadTaskCompleted || storedTask.TransferredSize != int64(len(content)) {
		t.Fatalf("stored task = %+v", storedTask)
	}
	replay, err := fixture.svc.BeginDownload(fixture.outsider.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatalf("replay error=%v", err)
	}
	var replayOutput bytes.Buffer
	if err := replay.WriteTo(context.Background(), &replayOutput); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replayOutput.Bytes(), content) {
		t.Fatalf("replay output=%q", replayOutput.Bytes())
	}
	old := fixture.svc.now().Add(-DownloadReplayWindow - time.Second).Unix()
	fixture.svc.db.Model(&model.DownloadTask{}).Where("id = ?", task.TaskID).Update("completed_at", old)
	if _, err := fixture.svc.BeginDownload(fixture.outsider.UserID, fixture.room.Code, task.TaskID); errx.Code(err) != errc.ErrDownloadExpired {
		t.Fatalf("expired replay error=%v", err)
	}
}

func TestDownloadRangeStreamsExactBytesAndOnlyEndRangeCompletes(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("0123456789")
	file := makeStoredAvailableFile(t, fixture, storage, "download-range", model.FileScopeShared, "range.txt", content, nil)
	task, err := fixture.svc.CreateDownloadTask(fixture.outsider.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	rangeSpec := ByteRange{Start: 2, End: 5}
	stream, err := fixture.svc.BeginDownloadRange(fixture.outsider.UserID, fixture.room.Code, task.TaskID, &rangeSpec)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := stream.WriteTo(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if string(output.Bytes()) != "2345" {
		t.Fatalf("range output=%q", output.Bytes())
	}
	var storedTask model.DownloadTask
	fixture.svc.db.First(&storedTask, "id = ?", task.TaskID)
	if storedTask.Status != model.DownloadTaskStreaming {
		t.Fatalf("middle range status=%q", storedTask.Status)
	}
	fixture.svc.db.Model(&model.DownloadTask{}).Where("id = ?", task.TaskID).Updates(map[string]interface{}{"started_at": fixture.svc.now().Add(-DownloadReplayWindow - time.Second).Unix()})
}

func TestPrivateAcceptCreatesDownloadAndCompletionUpdatesReceipt(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := makeStoredAvailableFile(t, fixture, storage, "download-private", model.FileScopeDirect, "私密.txt", []byte("private"), []string{fixture.recipient.UserID})
	if _, err := fixture.svc.CreateDownloadTask(fixture.recipient.UserID, fixture.room.Code, file.ID); errx.Code(err) != errc.ErrFileNotFound {
		t.Fatalf("pending recipient created task: %v", err)
	}
	task, err := fixture.svc.AcceptAndCreateDownload(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.WriteTo(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var recipient model.FileRecipient
	fixture.svc.db.Where("file_id = ? AND recipient_user_id = ?", file.ID, fixture.recipient.UserID).First(&recipient)
	if recipient.Status != model.RecipientDownloaded || recipient.DownloadCount != 1 || recipient.FirstDownloadedAt == nil || recipient.LastDownloadedAt == nil {
		t.Fatalf("recipient after download = %+v", recipient)
	}
	assertNotificationRecordCount(t, fixture.svc.db, NotificationTypeFileDownloaded, 1)
	replay, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.WriteTo(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	fixture.svc.db.Where("file_id = ? AND recipient_user_id = ?", file.ID, fixture.recipient.UserID).First(&recipient)
	if recipient.DownloadCount != 1 {
		t.Fatalf("replay changed download count=%d", recipient.DownloadCount)
	}
	assertNotificationRecordCount(t, fixture.svc.db, NotificationTypeFileDownloaded, 1)
	secondTask, err := fixture.svc.CreateDownloadTask(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondStream, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, secondTask.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := secondStream.WriteTo(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	assertNotificationRecordCount(t, fixture.svc.db, NotificationTypeFileDownloaded, 1)
}

type failingDownloadWriter struct{}

func (failingDownloadWriter) Write([]byte) (int, error) { return 0, errors.New("client disconnected") }

func TestFailedDownloadDoesNotCompleteOrRollbackAcceptance(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := makeStoredAvailableFile(t, fixture, storage, "download-failed", model.FileScopeDirect, "失败.txt", []byte("private"), []string{fixture.recipient.UserID})
	task, err := fixture.svc.AcceptAndCreateDownload(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.WriteTo(context.Background(), failingDownloadWriter{}); err == nil {
		t.Fatal("download unexpectedly succeeded")
	}
	var storedTask model.DownloadTask
	fixture.svc.db.First(&storedTask, "id = ?", task.TaskID)
	var recipient model.FileRecipient
	fixture.svc.db.Where("file_id = ? AND recipient_user_id = ?", file.ID, fixture.recipient.UserID).First(&recipient)
	if storedTask.Status != model.DownloadTaskFailed || recipient.Status != model.RecipientAccepted || recipient.DownloadCount != 0 {
		t.Fatalf("failed task=%+v recipient=%+v", storedTask, recipient)
	}
	assertNotificationRecordCount(t, fixture.svc.db, NotificationTypeFileDownloaded, 0)
}

func TestDownloadTaskIsBoundToCreatingUserAndExpires(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := makeStoredAvailableFile(t, fixture, storage, "download-bound", model.FileScopeShared, "bound.txt", []byte("data"), nil)
	task, err := fixture.svc.CreateDownloadTask(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.BeginDownload(fixture.outsider.UserID, fixture.room.Code, task.TaskID); errx.Code(err) != errc.ErrFileNotFound {
		t.Fatalf("other user task error=%v", err)
	}
	fixture.svc.db.Model(&model.DownloadTask{}).Where("id = ?", task.TaskID).Update("expires_at", fixture.svc.now().Add(-1).Unix())
	if _, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID); errx.Code(err) != errc.ErrDownloadExpired {
		t.Fatalf("expired task error=%v", err)
	}
}

func TestDownloadProgressIsOnlyPublishedToTaskUser(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	hub := NewStreamHub()
	fixture.svc.hub = hub
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := makeStoredAvailableFile(t, fixture, storage, "download-progress", model.FileScopeShared, "progress.txt", []byte("progress"), nil)
	userEvents, stopUser := hub.Subscribe(fixture.recipient.UserID)
	otherEvents, stopOther := hub.Subscribe(fixture.outsider.UserID)
	defer stopUser()
	defer stopOther()
	task, err := fixture.svc.CreateDownloadTask(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID)
	if err := stream.WriteTo(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	foundProgress := false
	for len(userEvents) > 0 {
		if event := <-userEvents; event.Type == "file.download_progress" {
			foundProgress = true
		}
	}
	if !foundProgress {
		t.Fatal("task user did not receive progress")
	}
	for len(otherEvents) > 0 {
		if event := <-otherEvents; event.Type == "file.download_progress" {
			t.Fatalf("other user received progress: %+v", event)
		}
	}
}

func TestDownloadRegistryEnforcesPerUserLimit(t *testing.T) {
	registry := &downloadRegistry{active: make(map[string]downloadActive), rooms: make(map[string]int), users: make(map[string]int)}
	_, finishOne, err := registry.begin("task-1", "room", "file-1", "user")
	if err != nil {
		t.Fatal(err)
	}
	defer finishOne()
	_, finishTwo, err := registry.begin("task-2", "room", "file-2", "user")
	if err != nil {
		t.Fatal(err)
	}
	defer finishTwo()
	if _, _, err := registry.begin("task-3", "room", "file-3", "user"); errx.Code(err) != errc.ErrDownloadLimited {
		t.Fatalf("third download error=%v", err)
	}
}

func TestLifecycleExpiresUnusedDownloadTask(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	now := fixture.svc.now()
	task := model.DownloadTask{ID: "expired-download", RoomID: fixture.room.ID, FileID: "file", UserID: fixture.recipient.UserID, ExpectedSize: 1, Status: model.DownloadTaskPending, CreatedAt: now.Add(-time.Hour).Unix(), ExpiresAt: now.Add(-time.Second).Unix()}
	if err := fixture.svc.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleSvc(fixture.svc.db, NewStreamHub())
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.expireDownloadTasks(now); err != nil {
		t.Fatal(err)
	}
	fixture.svc.db.First(&task, "id = ?", task.ID)
	if task.Status != model.DownloadTaskExpired {
		t.Fatalf("task status=%q", task.Status)
	}
}
