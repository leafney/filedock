package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

type fileTestFixture struct {
	svc       *FileSvc
	room      model.Room
	owner     model.RoomMember
	uploader  model.RoomMember
	recipient model.RoomMember
	outsider  model.RoomMember
}

func TestUploadContentStreamsSettlesAndStoresFile(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.svc.storage = storage
	content := []byte("hello")
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "upload-success", model.FileScopeShared, []FileManifest{{OriginalName: "hello.txt", DeclaredSize: int64(len(content)), DeclaredMIME: "text/plain"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fileID := batch.Files[0].ID
	if err := fixture.svc.UploadContent(context.Background(), fixture.uploader.UserID, fixture.room.Code, fileID, int64(len(content)), bytes.NewReader(content)); err != nil {
		t.Fatalf("UploadContent() error = %v", err)
	}
	var file model.RoomFile
	fixture.svc.db.First(&file, "id = ?", fileID)
	if file.Status != model.FileStatusAvailable || file.Progress != 100 || file.ActualSize != int64(len(content)) || file.DetectedMIME == "" {
		t.Fatalf("uploaded file = %+v", file)
	}
	stored, _, err := storage.Open(fixture.room.ID, file.StorageName)
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	read, _ := io.ReadAll(stored)
	if !bytes.Equal(read, content) {
		t.Fatalf("stored content = %q", read)
	}
}

func TestUploadContentSizeMismatchFailsAndReleasesReservation(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.svc.storage = storage
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "upload-short", model.FileScopeShared, []FileManifest{{OriginalName: "short.bin", DeclaredSize: 5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = fixture.svc.UploadContent(context.Background(), fixture.uploader.UserID, fixture.room.Code, batch.Files[0].ID, 5, bytes.NewReader([]byte("123")))
	if errx.Code(err) != errc.ErrUploadSize {
		t.Fatalf("UploadContent() code = %d, want %d: %v", errx.Code(err), errc.ErrUploadSize, err)
	}
	var room model.Room
	fixture.svc.db.First(&room, "id = ?", fixture.room.ID)
	if room.ReservedBytes != 0 || room.UsedBytes != 0 {
		t.Fatalf("capacity after failed upload = %+v", room)
	}
}

type controlledUploadReader struct {
	started chan struct{}
	resume  chan struct{}
	reads   int
}

func (r *controlledUploadReader) Read(buffer []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		close(r.started)
		buffer[0] = 'a'
		return 1, nil
	}
	if r.reads == 2 {
		<-r.resume
		buffer[0] = 'b'
		return 1, nil
	}
	return 0, io.EOF
}

func TestCancelActiveUploadStopsTaskAndReleasesReservation(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.svc.storage = storage
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "upload-cancel", model.FileScopeShared, []FileManifest{{OriginalName: "cancel.bin", DeclaredSize: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := &controlledUploadReader{started: make(chan struct{}), resume: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- fixture.svc.UploadContent(context.Background(), fixture.uploader.UserID, fixture.room.Code, batch.Files[0].ID, 2, reader)
	}()
	<-reader.started
	if err := fixture.svc.CancelUpload(fixture.uploader.UserID, fixture.room.Code, batch.Files[0].ID); err != nil {
		t.Fatal(err)
	}
	close(reader.resume)
	if err := <-done; !errors.Is(err, context.Canceled) && errx.Code(err) != errc.ErrUploadCancelled {
		t.Fatalf("active upload returned %v", err)
	}
	var file model.RoomFile
	fixture.svc.db.First(&file, "id = ?", batch.Files[0].ID)
	if file.Status != model.FileStatusCancelled {
		t.Fatalf("file status = %q", file.Status)
	}
}

func TestUploadRegistryEnforcesGlobalRoomAndUserLimits(t *testing.T) {
	registry := &uploadRegistry{active: make(map[string]uploadActive), rooms: make(map[string]int), users: make(map[string]int)}
	_, finishOne, err := registry.begin(context.Background(), "file-1", "room", "user")
	if err != nil {
		t.Fatal(err)
	}
	defer finishOne()
	_, finishTwo, err := registry.begin(context.Background(), "file-2", "room", "user")
	if err != nil {
		t.Fatal(err)
	}
	defer finishTwo()
	if _, _, err := registry.begin(context.Background(), "file-3", "room", "user"); errx.Code(err) != errc.ErrUploadLimited {
		t.Fatalf("third user upload error = %v", err)
	}
}

func newFileTestFixture(t *testing.T, capacity int64) fileTestFixture {
	t.Helper()
	_, db, _ := newSessionTestService(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	now := time.Now().Unix()
	room := model.Room{ID: "01FILETESTROOM00000000001", Code: "2468", Title: "room", OwnerUserID: "owner", JoinMode: model.JoinModeOpen, Status: model.RoomStatusActive, CreatedAt: now, ExpiresAt: now + 3600, CapacityBytes: capacity}
	members := []model.RoomMember{
		{ID: "member-owner", RoomID: room.ID, UserID: "owner", DisplayName: "Owner", Role: model.MemberRoleOwner, Status: model.MemberStatusActive, JoinedAt: now, LastSeenAt: now},
		{ID: "member-uploader", RoomID: room.ID, UserID: "uploader", DisplayName: "Uploader", Role: model.MemberRoleMember, Status: model.MemberStatusActive, JoinedAt: now, LastSeenAt: now},
		{ID: "member-recipient", RoomID: room.ID, UserID: "recipient", DisplayName: "Recipient", Role: model.MemberRoleMember, Status: model.MemberStatusActive, JoinedAt: now, LastSeenAt: now},
		{ID: "member-outsider", RoomID: room.ID, UserID: "outsider", DisplayName: "Outsider", Role: model.MemberRoleMember, Status: model.MemberStatusActive, JoinedAt: now, LastSeenAt: now},
	}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("create members: %v", err)
	}
	svc, err := NewFileSvc(db, nil)
	if err != nil {
		t.Fatalf("new file service: %v", err)
	}
	return fileTestFixture{svc: svc, room: room, owner: members[0], uploader: members[1], recipient: members[2], outsider: members[3]}
}

func TestCreateUploadBatchReservesCapacityAndIsIdempotent(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	manifest := []FileManifest{{OriginalName: "资料.pdf", DeclaredSize: 400, DeclaredMIME: "application/pdf"}, {OriginalName: "图片.png", DeclaredSize: 200, DeclaredMIME: "image/png"}}
	first, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "request-one", model.FileScopeDirect, manifest, []string{fixture.recipient.UserID})
	if err != nil {
		t.Fatalf("CreateUploadBatch() error = %v", err)
	}
	second, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "request-one", model.FileScopeDirect, manifest, []string{fixture.recipient.UserID})
	if err != nil {
		t.Fatalf("idempotent CreateUploadBatch() error = %v", err)
	}
	if first.Batch.ID != second.Batch.ID || len(first.Files) != 2 || len(second.Files) != 2 {
		t.Fatalf("idempotent batches differ: first=%+v second=%+v", first, second)
	}
	var room model.Room
	if err := fixture.svc.db.First(&room, "id = ?", fixture.room.ID).Error; err != nil {
		t.Fatal(err)
	}
	if room.ReservedBytes != 600 || room.UsedBytes != 0 {
		t.Fatalf("capacity = used %d reserved %d", room.UsedBytes, room.ReservedBytes)
	}
	var recipients int64
	if err := fixture.svc.db.Model(&model.FileRecipient{}).Count(&recipients).Error; err != nil || recipients != 2 {
		t.Fatalf("recipient count = %d, error %v", recipients, err)
	}
}

func TestCreateUploadBatchRejectsWholeBatchWhenCapacityInsufficient(t *testing.T) {
	fixture := newFileTestFixture(t, 500)
	_, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "too-large", model.FileScopeShared, []FileManifest{{OriginalName: "large.bin", DeclaredSize: 501}}, nil)
	if errx.Code(err) != errc.ErrRoomCapacity {
		t.Fatalf("error code = %d, want %d", errx.Code(err), errc.ErrRoomCapacity)
	}
	var count int64
	fixture.svc.db.Model(&model.RoomFile{}).Count(&count)
	if count != 0 {
		t.Fatalf("partial files created = %d", count)
	}
}

func TestConcurrentCapacityReservationAllowsOnlyOneBatch(t *testing.T) {
	fixture := newFileTestFixture(t, 100)
	start := make(chan struct{})
	errorsByRequest := make([]error, 2)
	var wait sync.WaitGroup
	for index := range errorsByRequest {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errorsByRequest[index] = fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "parallel-"+string(rune('a'+index)), model.FileScopeShared, []FileManifest{{OriginalName: "data.bin", DeclaredSize: 80}}, nil)
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	capacityErrors := 0
	for _, err := range errorsByRequest {
		if err == nil {
			successes++
		} else if errx.Code(err) == errc.ErrRoomCapacity {
			capacityErrors++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || capacityErrors != 1 {
		t.Fatalf("successes=%d capacityErrors=%d", successes, capacityErrors)
	}
}

func TestUploadCapacityStateTransitionsAreIdempotent(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "state", model.FileScopeShared, []FileManifest{{OriginalName: "ok.bin", DeclaredSize: 300}, {OriginalName: "cancel.bin", DeclaredSize: 200}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.MarkUploading(fixture.uploader.UserID, batch.Files[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.CompleteUpload(fixture.uploader.UserID, batch.Files[0].ID, "application/octet-stream", 300); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.ReleaseUpload(fixture.uploader.UserID, batch.Files[1].ID, model.FileStatusCancelled, errc.ErrUploadCancelled); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.ReleaseUpload(fixture.uploader.UserID, batch.Files[1].ID, model.FileStatusCancelled, errc.ErrUploadCancelled); err != nil {
		t.Fatalf("idempotent release: %v", err)
	}
	var room model.Room
	fixture.svc.db.First(&room, "id = ?", fixture.room.ID)
	if room.UsedBytes != 300 || room.ReservedBytes != 0 {
		t.Fatalf("capacity after transitions = used %d reserved %d", room.UsedBytes, room.ReservedBytes)
	}
}

func TestProjectFileProtectsUnrelatedPrivateMetadata(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	now := time.Now().Unix()
	file := model.RoomFile{ID: "file", RoomID: fixture.room.ID, UploaderUserID: fixture.uploader.UserID, OriginalName: "secret-plan.pdf", DeclaredMIME: "application/pdf", DetectedMIME: "application/pdf", DeclaredSize: 42, Scope: model.FileScopeDirect, PrivateCode: "7K2M-A9Q4", Status: model.FileStatusAvailable, Progress: 100, CreatedAt: now}
	recipients := []model.FileRecipient{{RecipientUserID: fixture.recipient.UserID, Status: model.RecipientPending}}
	members := map[string]model.RoomMember{fixture.recipient.UserID: fixture.recipient}

	uploader := ProjectFile(file, fixture.uploader, recipients, members, fixture.uploader)
	if uploader.Level != ProjectionFull || uploader.OriginalName != file.OriginalName || !uploader.Capabilities.CanReuse || !uploader.Capabilities.CanDownload {
		t.Fatalf("unexpected uploader projection: %+v", uploader)
	}
	recipient := ProjectFile(file, fixture.uploader, recipients, members, fixture.recipient)
	if recipient.Level != ProjectionFull || !recipient.Capabilities.CanAccept || recipient.Capabilities.CanDownload {
		t.Fatalf("unexpected recipient projection: %+v", recipient)
	}
	owner := ProjectFile(file, fixture.uploader, recipients, members, fixture.owner)
	if owner.Level != ProjectionAnonymous || owner.OriginalName != "" || owner.DeclaredMIME != "" || owner.DetectedMIME != "" || owner.Capabilities.CanDownload || owner.DisplayName != "7K2M-A9Q4" {
		t.Fatalf("private metadata leaked to owner: %+v", owner)
	}
	outsider := ProjectFile(file, fixture.uploader, recipients, members, fixture.outsider)
	if outsider.Level != ProjectionHidden || outsider.FileID != "" {
		t.Fatalf("private file leaked to outsider: %+v", outsider)
	}
}
