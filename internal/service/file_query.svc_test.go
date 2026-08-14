package service

import (
	"testing"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

func makeAvailableFile(t *testing.T, fixture fileTestFixture, key, scope, name string, size int64, recipients []string) model.RoomFile {
	t.Helper()
	batch, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, key, scope, []FileManifest{{OriginalName: name, DeclaredSize: size, DeclaredMIME: "application/octet-stream"}}, recipients)
	if err != nil {
		t.Fatal(err)
	}
	file := batch.Files[0]
	if err := fixture.svc.MarkUploading(fixture.uploader.UserID, file.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.CompleteUpload(fixture.uploader.UserID, file.ID, "application/octet-stream", size); err != nil {
		t.Fatal(err)
	}
	fixture.svc.db.First(&file, "id = ?", file.ID)
	return file
}

func TestListFilesAppliesSafeProjectionBeforeSearchAndCounts(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	shared := makeAvailableFile(t, fixture, "shared-list", model.FileScopeShared, "共享资料.pdf", 100, nil)
	direct := makeAvailableFile(t, fixture, "direct-list", model.FileScopeDirect, "绝密计划.pdf", 200, []string{fixture.recipient.UserID})

	uploaderList, err := fixture.svc.ListFiles(fixture.uploader.UserID, fixture.room.Code, FileListQuery{Range: FileRangeAll})
	if err != nil {
		t.Fatal(err)
	}
	if uploaderList.Total != 2 || len(uploaderList.Items) != 2 || uploaderList.Items[0].FileID != direct.ID || uploaderList.Items[1].FileID != shared.ID {
		t.Fatalf("uploader list = %+v", uploaderList)
	}
	ownerList, err := fixture.svc.ListFiles(fixture.owner.UserID, fixture.room.Code, FileListQuery{Range: FileRangeAll, Search: "绝密计划"})
	if err != nil {
		t.Fatal(err)
	}
	if ownerList.Total != 0 || len(ownerList.Items) != 0 {
		t.Fatalf("owner matched hidden original name: %+v", ownerList)
	}
	ownerByCode, err := fixture.svc.ListFiles(fixture.owner.UserID, fixture.room.Code, FileListQuery{Range: model.FileScopeDirect, Search: direct.PrivateCode})
	if err != nil || ownerByCode.Total != 1 || ownerByCode.Items[0].Level != ProjectionAnonymous || ownerByCode.Items[0].OriginalName != "" {
		t.Fatalf("owner anonymous list = %+v error=%v", ownerByCode, err)
	}
	outsiderList, err := fixture.svc.ListFiles(fixture.outsider.UserID, fixture.room.Code, FileListQuery{Range: FileRangeAll})
	if err != nil {
		t.Fatal(err)
	}
	if outsiderList.Total != 1 || len(outsiderList.Items) != 1 || outsiderList.Items[0].Scope != model.FileScopeShared {
		t.Fatalf("outsider list leaked direct file: %+v", outsiderList)
	}
}

func TestFileListReturnsAllFilesWithoutPagination(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	makeAvailableFile(t, fixture, "shared-page-1", model.FileScopeShared, "one", 10, nil)
	makeAvailableFile(t, fixture, "shared-page-2", model.FileScopeShared, "two", 20, nil)
	makeAvailableFile(t, fixture, "direct-page-1", model.FileScopeDirect, "private", 30, []string{fixture.recipient.UserID})
	result, err := fixture.svc.ListFiles(fixture.uploader.UserID, fixture.room.Code, FileListQuery{Range: FileRangeAll})
	if err != nil {
		t.Fatal(err)
	}
	if err != nil || result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("result = %+v error=%v", result, err)
	}
}

func TestPrivateRecipientStateReuseAndPublishShared(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "private-actions", model.FileScopeDirect, "private.bin", 100, []string{fixture.recipient.UserID})
	if err := fixture.svc.DeclineFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.AcceptFile(fixture.recipient.UserID, fixture.room.Code, file.ID); errx.Code(err) != errc.ErrFileRecipientState {
		t.Fatalf("accept after decline code=%d error=%v", errx.Code(err), err)
	}
	reused, err := fixture.svc.ReusePrivateFiles(fixture.uploader.UserID, fixture.room.Code, []string{file.ID}, []string{fixture.recipient.UserID, fixture.outsider.UserID})
	if err != nil || reused.Changed != 2 {
		t.Fatalf("reuse result=%+v error=%v", reused, err)
	}
	if err := fixture.svc.AcceptFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	recipientList, err := fixture.svc.ListFiles(fixture.recipient.UserID, fixture.room.Code, FileListQuery{Range: model.FileScopeDirect})
	if err != nil || !recipientList.Items[0].Capabilities.CanDownload || recipientList.Items[0].Capabilities.CanAccept {
		t.Fatalf("accepted projection=%+v error=%v", recipientList, err)
	}
	if err := fixture.svc.PublishShared(fixture.uploader.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	sharedList, err := fixture.svc.ListFiles(fixture.outsider.UserID, fixture.room.Code, FileListQuery{Range: model.FileScopeShared})
	if err != nil || sharedList.Total != 1 || sharedList.Items[0].OriginalName != "private.bin" {
		t.Fatalf("published list=%+v error=%v", sharedList, err)
	}
	var room model.Room
	fixture.svc.db.First(&room, "id = ?", fixture.room.ID)
	if room.UsedBytes != 100 || room.ReservedBytes != 0 {
		t.Fatalf("reuse or publish changed capacity: %+v", room)
	}
}

func TestFileEventsPersistAndRespectViewerVisibility(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "events", model.FileScopeDirect, "event-secret.bin", 100, []string{fixture.recipient.UserID})
	if err := fixture.svc.AcceptFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	ownerEvents, err := fixture.svc.ListFileEvents(fixture.owner.UserID, fixture.room.Code, "", 100)
	if err != nil || len(ownerEvents.Items) == 0 {
		t.Fatalf("owner events=%+v error=%v", ownerEvents, err)
	}
	for _, event := range ownerEvents.Items {
		if event.File != nil && event.File.FileID == file.ID && event.File.Level != ProjectionAnonymous {
			t.Fatalf("owner received full private event: %+v", event)
		}
	}
	outsiderEvents, err := fixture.svc.ListFileEvents(fixture.outsider.UserID, fixture.room.Code, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range outsiderEvents.Items {
		if event.File != nil && event.File.FileID == file.ID {
			t.Fatalf("outsider received private event: %+v", event)
		}
	}
}

func TestFileEventsAggregateOperationsAndTransferLifecycle(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "aggregate-events", model.FileScopeDirect, "aggregate-secret.bin", 100, []string{fixture.recipient.UserID})
	if _, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, ""); err != nil {
		t.Fatal(err)
	}
	uploaderEvents, err := fixture.svc.ListFileEvents(fixture.uploader.UserID, fixture.room.Code, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploaderEvents.Items) != 2 {
		t.Fatalf("expected upload and trash cards, got %+v", uploaderEvents.Items)
	}
	for _, item := range uploaderEvents.Items {
		if len(item.History) == 0 {
			t.Fatalf("card history is empty: %+v", item)
		}
	}

	// Restore the file, decline the original delivery, then reuse it. The
	// second delivery must create a new transfer operation instead of mutating
	// the original upload card.
	if _, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.DeclineFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.ReusePrivateFiles(fixture.uploader.UserID, fixture.room.Code, []string{file.ID}, []string{fixture.recipient.UserID}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.AcceptFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	uploaderEvents, err = fixture.svc.ListFileEvents(fixture.uploader.UserID, fixture.room.Code, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploaderEvents.Items) != 4 {
		t.Fatalf("expected four independent operation cards, got %+v", uploaderEvents.Items)
	}
	transferCards := 0
	for _, item := range uploaderEvents.Items {
		if item.OperationType != "transfer" {
			continue
		}
		transferCards++
		if item.RecipientSummary == nil || item.RecipientSummary.Accepted != 1 || item.RecipientSummary.Total != 1 {
			t.Fatalf("transfer summary = %+v", item)
		}
	}
	if transferCards != 1 {
		t.Fatalf("transfer cards = %d, events=%+v", transferCards, uploaderEvents.Items)
	}
	ownerEvents, err := fixture.svc.ListFileEvents(fixture.owner.UserID, fixture.room.Code, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range ownerEvents.Items {
		if item.RecipientSummary != nil && len(item.History) > 0 {
			for _, history := range item.History {
				if len(history.Recipients) > 0 {
					t.Fatalf("owner saw private recipient details: %+v", item)
				}
			}
		}
	}
}
