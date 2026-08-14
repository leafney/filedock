package service

import (
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

func TestListTrashAppliesVisibilityProjectionSearchAndCapabilities(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	selfDeleted := makeAvailableFile(t, fixture, "trash-list-self", model.FileScopeShared, "公开资料.txt", 100, nil)
	ownerDeleted := makeAvailableFile(t, fixture, "trash-list-owner", model.FileScopeDirect, "绝密计划.txt", 200, []string{fixture.recipient.UserID})

	activeOwner, err := fixture.svc.ListFiles(fixture.owner.UserID, fixture.room.Code, FileListQuery{Range: model.FileScopeDirect})
	if err != nil {
		t.Fatal(err)
	}
	ownerActiveFile := activeOwner.Items[0]
	if ownerActiveFile.Level != ProjectionAnonymous || !ownerActiveFile.Capabilities.CanTrash || !ownerActiveFile.Capabilities.CanSetTrashReason {
		t.Fatalf("owner active projection=%+v", ownerActiveFile)
	}
	if _, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, selfDeleted.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, ownerDeleted.ID, "  capacity cleanup  "); err != nil {
		t.Fatal(err)
	}

	memberPage, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{})
	if err != nil || memberPage.Total != 2 {
		t.Fatalf("member page=%+v error=%v", memberPage, err)
	}
	items := trashItemsByID(memberPage.Items)
	if !items[selfDeleted.ID].Capabilities.CanRestore || !items[selfDeleted.ID].Capabilities.CanPurge || items[selfDeleted.ID].Capabilities.CanRequestRestore {
		t.Fatalf("self-deleted capabilities=%+v", items[selfDeleted.ID].Capabilities)
	}
	ownerItem := items[ownerDeleted.ID]
	if ownerItem.File.Level != ProjectionFull || ownerItem.DeleteReason != "capacity cleanup" || !ownerItem.Capabilities.CanRequestRestore || ownerItem.Capabilities.CanRestore || ownerItem.Capabilities.CanPurge {
		t.Fatalf("owner-deleted member projection=%+v", ownerItem)
	}

	ownerByOriginal, err := fixture.svc.ListTrash(fixture.owner.UserID, fixture.room.Code, FileTrashListQuery{Search: "绝密计划"})
	if err != nil || ownerByOriginal.Total != 0 {
		t.Fatalf("anonymous original search=%+v error=%v", ownerByOriginal, err)
	}
	ownerByCode, err := fixture.svc.ListTrash(fixture.owner.UserID, fixture.room.Code, FileTrashListQuery{Search: ownerDeleted.PrivateCode})
	if err != nil || ownerByCode.Total != 1 || ownerByCode.Items[0].File.Level != ProjectionAnonymous || ownerByCode.Items[0].File.OriginalName != "" {
		t.Fatalf("anonymous code search=%+v error=%v", ownerByCode, err)
	}
	if !ownerByCode.Items[0].Capabilities.CanRestore || !ownerByCode.Items[0].Capabilities.CanPurge || ownerByCode.Items[0].Capabilities.CanRequestRestore {
		t.Fatalf("owner trash capabilities=%+v", ownerByCode.Items[0].Capabilities)
	}
	recipientPage, err := fixture.svc.ListTrash(fixture.recipient.UserID, fixture.room.Code, FileTrashListQuery{})
	if err != nil || recipientPage.Total != 0 {
		t.Fatalf("recipient page leaked file=%+v error=%v", recipientPage, err)
	}
}

func TestListTrashProjectsRestoreRequestAndRejection(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "trash-list-request", model.FileScopeShared, "申请恢复.txt", 100, nil)
	if _, err := fixture.svc.TrashFile(fixture.owner.UserID, fixture.room.Code, file.ID, "outdated"); err != nil {
		t.Fatal(err)
	}
	action, err := fixture.svc.RestoreFile(fixture.uploader.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	ownerPage, err := fixture.svc.ListTrash(fixture.owner.UserID, fixture.room.Code, FileTrashListQuery{})
	if err != nil || ownerPage.Total != 1 || ownerPage.Items[0].RestoreRequest == nil {
		t.Fatalf("owner request page=%+v error=%v", ownerPage, err)
	}
	request := ownerPage.Items[0].RestoreRequest
	if request.RequestID != action.RequestID || request.Status != model.FileRestorePending || request.RequesterName != fixture.uploader.DisplayName {
		t.Fatalf("request projection=%+v", request)
	}
	if err := fixture.svc.RejectFileRestore(fixture.owner.UserID, fixture.room.Code, action.RequestID, "  keep removed  "); err != nil {
		t.Fatal(err)
	}
	memberPage, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{})
	if err != nil || memberPage.Items[0].RestoreRequest == nil {
		t.Fatalf("member rejected page=%+v error=%v", memberPage, err)
	}
	rejected := memberPage.Items[0]
	if rejected.RestoreRequest.Status != model.FileRestoreRejected || rejected.RestoreRequest.RejectionReason != "keep removed" || rejected.Capabilities.CanRequestRestore {
		t.Fatalf("rejected projection=%+v", rejected)
	}
}

func TestListTrashUsesDeletionOrderStableCursorAndMembership(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	base := time.Unix(1_800_000_000, 0)
	files := make([]model.RoomFile, 0, 3)
	for index := 0; index < 3; index++ {
		file := makeAvailableFile(t, fixture, "trash-page-"+string(rune('a'+index)), model.FileScopeShared, "page.txt", 10, nil)
		fixture.svc.now = func() time.Time { return base.Add(time.Duration(index) * time.Second) }
		if _, err := fixture.svc.TrashFile(fixture.uploader.UserID, fixture.room.Code, file.ID, ""); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	first, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{Limit: 2})
	if err != nil || first.Total != 3 || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%+v error=%v", first, err)
	}
	if first.Items[0].File.FileID != files[2].ID || first.Items[1].File.FileID != files[1].ID {
		t.Fatalf("deletion order=%+v", first.Items)
	}
	second, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{Limit: 2, Cursor: first.NextCursor})
	if err != nil || second.Total != 3 || len(second.Items) != 1 || second.Items[0].File.FileID != files[0].ID || second.NextCursor != "" {
		t.Fatalf("second page=%+v error=%v", second, err)
	}

	if err := fixture.svc.db.Model(&model.RoomMember{}).Where("id = ?", fixture.uploader.ID).Update("status", model.MemberStatusLeft).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{}); errx.Code(err) != errc.ErrMemberRequired {
		t.Fatalf("left member error=%v", err)
	}
	if err := fixture.svc.db.Model(&model.RoomMember{}).Where("id = ?", fixture.uploader.ID).Update("status", model.MemberStatusActive).Error; err != nil {
		t.Fatal(err)
	}
	rejoined, err := fixture.svc.ListTrash(fixture.uploader.UserID, fixture.room.Code, FileTrashListQuery{})
	if err != nil || rejoined.Total != 3 {
		t.Fatalf("rejoined page=%+v error=%v", rejoined, err)
	}
}

func trashItemsByID(items []FileTrashProjection) map[string]FileTrashProjection {
	result := make(map[string]FileTrashProjection, len(items))
	for _, item := range items {
		result[item.File.FileID] = item
	}
	return result
}
