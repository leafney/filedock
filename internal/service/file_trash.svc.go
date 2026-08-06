package service

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const MaxFileLifecycleReasonRunes = 200

const (
	FileRestoreActionRestored = "restored"
	FileRestoreActionPending  = "pending"
)

type FileRestoreActionResult struct {
	Status    string `json:"status"`
	RequestID string `json:"requestId,omitempty"`
}

// TrashFile starts a new deletion generation. Storage deletion, active stream
// cancellation and lifecycle notifications are coordinated by later phases;
// this method owns the authoritative database state transition.
func (s *FileSvc) TrashFile(userID, roomCode, fileID, reason string) (model.FileTrashCycle, error) {
	reason, err := normalizeFileLifecycleReason(reason)
	if err != nil {
		return model.FileTrashCycle{}, err
	}
	var cycle model.FileTrashCycle
	err = s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, userID)
		if err != nil {
			return err
		}
		var file model.RoomFile
		if err := tx.Where("id = ? AND room_id = ? AND status = ?", fileID, room.ID, model.FileStatusAvailable).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		if member.Role != model.MemberRoleOwner && file.UploaderUserID != userID {
			return errx.New(errc.ErrFileNotFound, nil)
		}
		if file.UploaderUserID == userID && reason != "" {
			return errx.New(errc.ErrFileManifest, nil)
		}
		version := file.TrashVersion + 1
		update := tx.Model(&model.RoomFile{}).
			Where("id = ? AND room_id = ? AND status = ? AND trash_version = ?", file.ID, room.ID, model.FileStatusAvailable, file.TrashVersion).
			Updates(map[string]interface{}{"status": model.FileStatusTrashed, "trash_version": version})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		cycleID, err := ulidx.New()
		if err != nil {
			return err
		}
		now := s.now().Unix()
		cycle = model.FileTrashCycle{ID: cycleID, RoomID: room.ID, FileID: file.ID, Version: version, DeletedByUserID: userID, DeletedAt: now, DeleteReason: reason, Outcome: model.FileTrashActive}
		if err := tx.Create(&cycle).Error; err != nil {
			return err
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventTrashed, now)
	})
	return cycle, err
}

// RestoreFile either restores immediately or creates/reactivates the single
// restore request for the current deletion generation.
func (s *FileSvc) RestoreFile(userID, roomCode, fileID string) (FileRestoreActionResult, error) {
	var result FileRestoreActionResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, userID)
		if err != nil {
			return err
		}
		file, cycle, err := loadCurrentTrash(tx, room.ID, fileID)
		if err != nil {
			return err
		}
		if member.Role == model.MemberRoleOwner || (file.UploaderUserID == userID && cycle.DeletedByUserID == userID) {
			if err := restoreTrashCycle(tx, file, cycle, userID, nil, s.now().Unix()); err != nil {
				return err
			}
			result.Status = FileRestoreActionRestored
			return nil
		}
		if file.UploaderUserID != userID {
			return errx.New(errc.ErrFileNotFound, nil)
		}
		request, err := upsertRestoreRequest(tx, room.ID, file, cycle, userID, s.now().Unix())
		if err != nil {
			return err
		}
		result = FileRestoreActionResult{Status: FileRestoreActionPending, RequestID: request.ID}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventRestoreRequested, request.CreatedAt)
	})
	return result, err
}

func (s *FileSvc) ApproveFileRestore(ownerID, roomCode, requestID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, ownerID)
		if err != nil {
			return err
		}
		if member.Role != model.MemberRoleOwner || room.OwnerUserID != ownerID {
			return errx.New(errc.ErrOwnerRequired, nil)
		}
		var request model.FileRestoreRequest
		if err := tx.Where("id = ? AND room_id = ? AND status = ?", requestID, room.ID, model.FileRestorePending).First(&request).Error; err != nil {
			return restoreRequestStateError(err)
		}
		file, cycle, err := loadCurrentTrash(tx, room.ID, request.FileID)
		if err != nil || cycle.ID != request.TrashCycleID || cycle.Version != request.TrashVersion {
			if err != nil && errx.Code(err) == 0 {
				return err
			}
			return errx.New(errc.ErrFileState, nil)
		}
		now := s.now().Unix()
		return restoreTrashCycle(tx, file, cycle, ownerID, &request, now)
	})
}

func (s *FileSvc) RejectFileRestore(ownerID, roomCode, requestID, reason string) error {
	reason, err := normalizeFileLifecycleReason(reason)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, ownerID)
		if err != nil {
			return err
		}
		if member.Role != model.MemberRoleOwner || room.OwnerUserID != ownerID {
			return errx.New(errc.ErrOwnerRequired, nil)
		}
		var request model.FileRestoreRequest
		if err := tx.Where("id = ? AND room_id = ? AND status = ?", requestID, room.ID, model.FileRestorePending).First(&request).Error; err != nil {
			return restoreRequestStateError(err)
		}
		file, cycle, err := loadCurrentTrash(tx, room.ID, request.FileID)
		if err != nil || cycle.ID != request.TrashCycleID || cycle.Version != request.TrashVersion {
			if err != nil && errx.Code(err) == 0 {
				return err
			}
			return errx.New(errc.ErrFileState, nil)
		}
		now := s.now().Unix()
		update := tx.Model(&model.FileRestoreRequest{}).
			Where("id = ? AND status = ?", request.ID, model.FileRestorePending).
			Updates(map[string]interface{}{"status": model.FileRestoreRejected, "decided_at": now, "decided_by_user_id": ownerID, "rejection_reason": reason})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, ownerID, FileEventRestoreRejected, now)
	})
}

// InvalidateMemberRestoreRequests is called by room membership transitions in
// the same database transaction. Rejoining may reactivate invalidated rows.
func (s *FileSvc) InvalidateMemberRestoreRequests(tx *gorm.DB, roomID, userID string, now int64) error {
	if s == nil || s.db == nil {
		return nil
	}
	if tx == nil {
		tx = s.db
	}
	return tx.Model(&model.FileRestoreRequest{}).
		Where("room_id = ? AND requester_user_id = ? AND status = ?", roomID, userID, model.FileRestorePending).
		Updates(map[string]interface{}{"status": model.FileRestoreInvalidated, "decided_at": now, "decided_by_user_id": "", "rejection_reason": ""}).Error
}

func loadCurrentTrash(tx *gorm.DB, roomID, fileID string) (model.RoomFile, model.FileTrashCycle, error) {
	var file model.RoomFile
	if err := tx.Where("id = ? AND room_id = ? AND status = ?", fileID, roomID, model.FileStatusTrashed).First(&file).Error; err != nil {
		return file, model.FileTrashCycle{}, fileNotFound(err)
	}
	var cycle model.FileTrashCycle
	if err := tx.Where("file_id = ? AND version = ? AND outcome = ?", file.ID, file.TrashVersion, model.FileTrashActive).First(&cycle).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return file, cycle, errx.New(errc.ErrFileState, nil)
		}
		return file, cycle, err
	}
	return file, cycle, nil
}

func restoreTrashCycle(tx *gorm.DB, file model.RoomFile, cycle model.FileTrashCycle, actorID string, request *model.FileRestoreRequest, now int64) error {
	fileUpdate := tx.Model(&model.RoomFile{}).
		Where("id = ? AND status = ? AND trash_version = ?", file.ID, model.FileStatusTrashed, cycle.Version).
		Update("status", model.FileStatusAvailable)
	if fileUpdate.Error != nil {
		return fileUpdate.Error
	}
	if fileUpdate.RowsAffected != 1 {
		return errx.New(errc.ErrFileState, nil)
	}
	cycleUpdate := tx.Model(&model.FileTrashCycle{}).
		Where("id = ? AND outcome = ?", cycle.ID, model.FileTrashActive).
		Updates(map[string]interface{}{"outcome": model.FileTrashRestored, "resolved_by_user_id": actorID, "resolved_at": now})
	if cycleUpdate.Error != nil {
		return cycleUpdate.Error
	}
	if cycleUpdate.RowsAffected != 1 {
		return errx.New(errc.ErrFileState, nil)
	}
	if request == nil {
		var pending model.FileRestoreRequest
		if err := tx.Where("trash_cycle_id = ? AND status = ?", cycle.ID, model.FileRestorePending).First(&pending).Error; err == nil {
			request = &pending
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if request != nil {
		requestUpdate := tx.Model(&model.FileRestoreRequest{}).
			Where("id = ? AND status = ?", request.ID, model.FileRestorePending).
			Updates(map[string]interface{}{"status": model.FileRestoreApproved, "decided_at": now, "decided_by_user_id": actorID, "rejection_reason": ""})
		if requestUpdate.Error != nil {
			return requestUpdate.Error
		}
		if requestUpdate.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
	}
	return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, actorID, FileEventRestored, now)
}

func upsertRestoreRequest(tx *gorm.DB, roomID string, file model.RoomFile, cycle model.FileTrashCycle, requesterID string, now int64) (model.FileRestoreRequest, error) {
	var request model.FileRestoreRequest
	err := tx.Where("trash_cycle_id = ?", cycle.ID).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		requestID, idErr := ulidx.New()
		if idErr != nil {
			return request, idErr
		}
		request = model.FileRestoreRequest{ID: requestID, RoomID: roomID, FileID: file.ID, TrashCycleID: cycle.ID, TrashVersion: cycle.Version, RequesterUserID: requesterID, Status: model.FileRestorePending, CreatedAt: now}
		return request, tx.Create(&request).Error
	}
	if err != nil {
		return request, err
	}
	if request.RequesterUserID != requesterID || request.Status != model.FileRestoreInvalidated {
		return request, errx.New(errc.ErrFileState, nil)
	}
	update := tx.Model(&model.FileRestoreRequest{}).
		Where("id = ? AND status = ?", request.ID, model.FileRestoreInvalidated).
		Updates(map[string]interface{}{"status": model.FileRestorePending, "created_at": now, "decided_at": nil, "decided_by_user_id": "", "rejection_reason": ""})
	if update.Error != nil {
		return request, update.Error
	}
	if update.RowsAffected != 1 {
		return request, errx.New(errc.ErrFileState, nil)
	}
	request.Status = model.FileRestorePending
	request.CreatedAt = now
	request.DecidedAt = nil
	request.DecidedByUserID = ""
	request.RejectionReason = ""
	return request, nil
}

func normalizeFileLifecycleReason(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len([]rune(value)) > MaxFileLifecycleReasonRunes {
		return "", errx.New(errc.ErrFileManifest, nil)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", errx.New(errc.ErrFileManifest, nil)
		}
	}
	return value, nil
}

func restoreRequestStateError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errx.New(errc.ErrFileState, nil)
	}
	return err
}
