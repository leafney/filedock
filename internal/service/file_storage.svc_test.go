package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
)

func TestFileStorageStreamsAndOpensInternalFile(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("hello filedock")
	var progresses []int64
	stored, err := storage.Write(context.Background(), "room-01", "storage-01", int64(len(content)), bytes.NewReader(content), func(value int64) {
		progresses = append(progresses, value)
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if stored.Size != int64(len(content)) || stored.DetectedMIME == "" || filepath.Base(stored.Path) != "storage-01" || len(progresses) == 0 {
		t.Fatalf("unexpected stored file: %+v progress=%v", stored, progresses)
	}
	file, info, err := storage.Open("room-01", "storage-01")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	read, err := io.ReadAll(file)
	if err != nil || !bytes.Equal(read, content) || info.Size() != int64(len(content)) {
		t.Fatalf("opened content=%q size=%d error=%v", read, info.Size(), err)
	}
}

func TestFileStorageRejectsSizeMismatchAndCleansTemporaryFile(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, testCase := range map[string]struct {
		content  []byte
		declared int64
	}{
		"short": {content: []byte("12"), declared: 3},
		"long":  {content: []byte("1234"), declared: 3},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := storage.Write(context.Background(), "room-02", name, testCase.declared, bytes.NewReader(testCase.content), nil)
			if err == nil {
				t.Fatal("Write() unexpectedly succeeded")
			}
			path := filepath.Join(storage.uploadRoot, "room-02", name+".part")
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("temporary file remains: %v", statErr)
			}
		})
	}
}

func TestFileStorageRejectsEscapesAndSymbolicLinks(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Write(context.Background(), "../escape", "name", 1, bytes.NewReader([]byte("x")), nil); err == nil {
		t.Fatal("path escape unexpectedly succeeded")
	}
	external := t.TempDir()
	link := filepath.Join(storage.roomsRoot, "linked-room")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if _, err := storage.Write(context.Background(), "linked-room", "name", 1, bytes.NewReader([]byte("x")), nil); err == nil {
		t.Fatal("symbolic link room unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(external, "name")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file escaped into symbolic link target")
	}
}

func TestFileStorageHonorsCancellation(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := storage.Write(ctx, "room-03", "cancelled", 3, bytes.NewReader([]byte("123")), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Write() error = %v, want context canceled", err)
	}
}

func TestFileStorageWritesPartsOutOfOrderAndFinalizes(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.PrepareUpload("room-parts", "storage-parts", 6); err != nil {
		t.Fatal(err)
	}
	if err := storage.WritePart(context.Background(), "room-parts", "storage-parts", 3, 3, 6, bytes.NewReader([]byte("def"))); err != nil {
		t.Fatal(err)
	}
	if err := storage.WritePart(context.Background(), "room-parts", "storage-parts", 0, 3, 6, bytes.NewReader([]byte("abc"))); err != nil {
		t.Fatal(err)
	}
	stored, err := storage.FinalizeUpload("room-parts", "storage-parts", 6)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Size != 6 || stored.DetectedMIME == "" {
		t.Fatalf("stored=%+v", stored)
	}
	file, _, err := storage.Open("room-parts", "storage-parts")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil || string(content) != "abcdef" {
		t.Fatalf("content=%q error=%v", content, err)
	}
}

func TestFileStorageRejectsShortUploadPart(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = storage.WritePart(context.Background(), "room-parts-short", "storage-parts-short", 0, 3, 3, bytes.NewReader([]byte("ab")))
	if err == nil {
		t.Fatal("WritePart() unexpectedly succeeded")
	}
	if err := storage.DeleteTemporaryFile("room-parts-short", "storage-parts-short"); err != nil {
		t.Fatal(err)
	}
}

func TestFileStorageDeletesTemporaryUpload(t *testing.T) {
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.PrepareUpload("room-parts-delete", "storage-parts-delete", 3); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteTemporaryFile("room-parts-delete", "storage-parts-delete"); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteTemporaryFile("room-parts-delete", "storage-parts-delete"); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleCleanupDeletesFileStorageAndMetadata(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "cleanup", model.FileScopeShared, []FileManifest{{OriginalName: "cleanup.bin", DeclaredSize: 3}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := batch.Files[0]
	if _, err := storage.Write(context.Background(), fixture.room.ID, file.StorageName, 3, bytes.NewReader([]byte("123")), nil); err != nil {
		t.Fatal(err)
	}
	job := model.CleanupJob{ID: "cleanup-job", RoomID: fixture.room.ID, Status: model.CleanupRunning, Phase: "running", ScheduledAt: time.Now().Unix()}
	if err := fixture.svc.db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleSvc(fixture.svc.db, NewStreamHub(), storage)
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.runCleanup(fixture.room.ID, job.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.Open(fixture.room.ID, file.StorageName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stored file still available: %v", err)
	}
	var files, batches, recipients, events int64
	fixture.svc.db.Model(&model.RoomFile{}).Count(&files)
	fixture.svc.db.Model(&model.UploadBatch{}).Count(&batches)
	fixture.svc.db.Model(&model.FileRecipient{}).Count(&recipients)
	fixture.svc.db.Model(&model.FileEvent{}).Count(&events)
	if files+batches+recipients+events != 0 {
		t.Fatalf("file metadata remains: files=%d batches=%d recipients=%d events=%d", files, batches, recipients, events)
	}
}

func TestLifecycleCleanupKeepsMetadataWhenStorageDeletionFails(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "cleanup-failure", model.FileScopeShared, []FileManifest{{OriginalName: "kept.bin", DeclaredSize: 3}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(storage.roomsRoot, fixture.room.ID)); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	job := model.CleanupJob{ID: "cleanup-failed-job", RoomID: fixture.room.ID, Status: model.CleanupRunning, Phase: "running", ScheduledAt: time.Now().Unix()}
	if err := fixture.svc.db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleSvc(fixture.svc.db, NewStreamHub(), storage)
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.runCleanup(fixture.room.ID, job.ID, time.Now()); err == nil {
		t.Fatal("runCleanup() unexpectedly succeeded")
	}
	var count int64
	fixture.svc.db.Model(&model.RoomFile{}).Where("id = ?", batch.Files[0].ID).Count(&count)
	if count != 1 {
		t.Fatal("file metadata was removed after storage cleanup failure")
	}
}
