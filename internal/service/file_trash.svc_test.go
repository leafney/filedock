package service

import (
	"strings"
	"sync"
	"testing"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

func TestTrashFileEnforcesOwnershipAndStartsGeneration(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "trash-owner", model.FileScopeShared, "report.txt", 100, nil)

	if _, err := fixture.svc.TrashFile(fixture.outsider.UserID, fixture.room.Code, file.ID, ""); errx.Code(err) != errc.ErrFileNotFound {
		t.Fatalf("outsider trash code=%d error=%v", errx.Code(err), err)
	}
	if _, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, "self reason"); errx.Code(err) != errc.ErrFileManifest {
		t.Fatalf("self reason code=%d error=%v", errx.Code(err), err)
	}
	cycle, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if cycle.Version != 1 || cycle.DeletedByUserID != fixture.uploader.UserID || cycle.Outcome != model.FileTrashActive {
		t.Fatalf("cycle=%+v", cycle)
	}
	var stored model.RoomFile
	if err := fixture.svc.db.First(&stored, "id = ?", file.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.FileStatusTrashed || stored.TrashVersion != 1 {
		t.Fatalf("file=%+v", stored)
	}
	if _, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, ""); errx.Code(err) != errc.ErrFileNotFound {
		t.Fatalf("duplicate trash code=%d error=%v", errx.Code(err), err)
	}
}

func TestOwnerTrashReasonAndPrivateStateRestoreUnchanged(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "trash-private", model.FileScopeDirect, "secret.bin", 100, []string{fixture.recipient.UserID})
	if err := fixture.svc.AcceptFile(fixture.recipient.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	cycle, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, file.ID, "  policy cleanup  ")
	if err != nil {
		t.Fatal(err)
	}
	if cycle.DeleteReason != "policy cleanup" || cycle.DeletedByUserID != fixture.owner.UserID {
		t.Fatalf("cycle=%+v", cycle)
	}
	result, err := fixture.svc.RestoreFile(fixture.owner.UserID, fixture.room.Code, file.ID)
	if err != nil || result.Status != FileRestoreActionRestored {
		t.Fatalf("restore=%+v error=%v", result, err)
	}
	var recipient model.FileRecipient
	if err := fixture.svc.db.Where("file_id = ? AND recipient_user_id = ?", file.ID, fixture.recipient.UserID).First(&recipient).Error; err != nil {
		t.Fatal(err)
	}
	if recipient.Status != model.RecipientAccepted || recipient.DeliveryVersion != 1 {
		t.Fatalf("recipient changed=%+v", recipient)
	}
	var stored model.RoomFile
	fixture.svc.db.First(&stored, "id = ?", file.ID)
	if stored.Status != model.FileStatusAvailable || stored.TrashVersion != 1 {
		t.Fatalf("restored file=%+v", stored)
	}
}

func TestRestoreRequestRejectsRepeatUntilNewGeneration(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "trash-request", model.FileScopeShared, "request.txt", 100, nil)
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, file.ID, "obsolete"); err != nil {
		t.Fatal(err)
	}
	request, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID)
	if err != nil || request.Status != FileRestoreActionPending || request.RequestID == "" {
		t.Fatalf("request=%+v error=%v", request, err)
	}
	if _, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID); errx.Code(err) != errc.ErrFileState {
		t.Fatalf("duplicate request code=%d error=%v", errx.Code(err), err)
	}
	if err := fixture.svc.RejectFileRestore(fixture.owner.UserID, fixture.room.Code, request.RequestID, "  keep removed  "); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID); errx.Code(err) != errc.ErrFileState {
		t.Fatalf("request after reject code=%d error=%v", errx.Code(err), err)
	}
	var rejected model.FileRestoreRequest
	fixture.svc.db.First(&rejected, "id = ?", request.RequestID)
	if rejected.Status != model.FileRestoreRejected || rejected.RejectionReason != "keep removed" {
		t.Fatalf("rejected=%+v", rejected)
	}
	if _, err := fixture.svc.RestoreFile(fixture.owner.UserID, fixture.room.Code, file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, file.ID, "again"); err != nil {
		t.Fatal(err)
	}
	next, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID)
	if err != nil || next.Status != FileRestoreActionPending || next.RequestID == request.RequestID {
		t.Fatalf("next request=%+v error=%v", next, err)
	}
}

func TestApproveAndDirectOwnerRestoreResolvePendingRequest(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	first := makeAvailableFile(t, fixture, "approve-request", model.FileScopeShared, "approve.txt", 100, nil)
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, first.ID, ""); err != nil {
		t.Fatal(err)
	}
	request, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.ApproveFileRestore(fixture.owner.UserID, fixture.room.Code, request.RequestID); err != nil {
		t.Fatal(err)
	}
	assertRestoreRequestStatus(t, fixture, request.RequestID, model.FileRestoreApproved)

	second := makeAvailableFile(t, fixture, "direct-request", model.FileScopeShared, "direct.txt", 100, nil)
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, second.ID, ""); err != nil {
		t.Fatal(err)
	}
	directRequest, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.RestoreFile(fixture.owner.UserID, fixture.room.Code, second.ID); err != nil {
		t.Fatal(err)
	}
	assertRestoreRequestStatus(t, fixture, directRequest.RequestID, model.FileRestoreApproved)
}

func TestInvalidatedRestoreRequestCanReactivateButRejectedCannot(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "invalidate-request", model.FileScopeShared, "leave.txt", 100, nil)
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, file.ID, ""); err != nil {
		t.Fatal(err)
	}
	request, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.InvalidateMemberRestoreRequests(nil, fixture.room.ID, fixture.uploader.UserID, fixture.svc.now().Unix()); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID)
	if err != nil || result.RequestID != request.RequestID {
		t.Fatalf("reactivated=%+v error=%v", result, err)
	}
	if err := fixture.svc.RejectFileRestore(fixture.owner.UserID, fixture.room.Code, request.RequestID, "no"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.svc.InvalidateMemberRestoreRequests(nil, fixture.room.ID, fixture.uploader.UserID, fixture.svc.now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID); errx.Code(err) != errc.ErrFileState {
		t.Fatalf("rejected reactivation code=%d error=%v", errx.Code(err), err)
	}
}

func TestTrashLifecycleReasonValidation(t *testing.T) {
	if value, err := normalizeFileLifecycleReason("  reason  "); err != nil || value != "reason" {
		t.Fatalf("normalized=%q error=%v", value, err)
	}
	for _, value := range []string{strings.Repeat("界", MaxFileLifecycleReasonRunes+1), "line\nbreak"} {
		if _, err := normalizeFileLifecycleReason(value); errx.Code(err) != errc.ErrFileManifest {
			t.Fatalf("invalid reason code=%d error=%v", errx.Code(err), err)
		}
	}
}

func TestConcurrentTrashTransitionOnlySucceedsOnce(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "trash-concurrent", model.FileScopeShared, "parallel.txt", 100, nil)
	start := make(chan struct{})
	errorsByCall := make([]error, 2)
	var wait sync.WaitGroup
	for index := range errorsByCall {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errorsByCall[index] = fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, "")
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	failures := 0
	for _, err := range errorsByCall {
		if err == nil {
			successes++
		} else if errx.Code(err) == errc.ErrFileNotFound || errx.Code(err) == errc.ErrFileState {
			failures++
		} else {
			t.Fatalf("unexpected error=%v", err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("successes=%d failures=%d", successes, failures)
	}
}

func assertRestoreRequestStatus(t *testing.T, fixture fileTestFixture, requestID, want string) {
	t.Helper()
	var request model.FileRestoreRequest
	if err := fixture.svc.db.First(&request, "id = ?", requestID).Error; err != nil {
		t.Fatal(err)
	}
	if request.Status != want || request.DecidedAt == nil {
		t.Fatalf("request=%+v want status=%s", request, want)
	}
}
