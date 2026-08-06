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
	var cancelledTasks []model.DownloadTask
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
			return errx.New(errc.ErrFileLifecycleReason, nil)
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
		if err := tx.Where("file_id = ? AND status IN ?", file.ID, []string{model.DownloadTaskPending, model.DownloadTaskStreaming}).Find(&cancelledTasks).Error; err != nil {
			return err
		}
		if len(cancelledTasks) > 0 {
			if err := tx.Model(&model.DownloadTask{}).
				Where("file_id = ? AND status IN ?", file.ID, []string{model.DownloadTaskPending, model.DownloadTaskStreaming}).
				Updates(map[string]interface{}{"status": model.DownloadTaskCancelled, "cancelled_at": now}).Error; err != nil {
				return err
			}
		}
		cycle = model.FileTrashCycle{ID: cycleID, RoomID: room.ID, FileID: file.ID, Version: version, DeletedByUserID: userID, DeletedAt: now, DeleteReason: reason, Outcome: model.FileTrashActive}
		if err := tx.Create(&cycle).Error; err != nil {
			return err
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventTrashed, now)
	})
	if err == nil {
		s.downloads.cancelFile(fileID)
		if s.hub != nil {
			for _, task := range cancelledTasks {
				s.hub.PublishUser(task.UserID, "file.download_cancelled", map[string]interface{}{"roomCode": roomCode, "fileId": fileID, "taskId": task.ID})
			}
		}
		s.publishFileProjection(fileID, "file.trashed", nil)
	}
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
	if err == nil {
		eventType := "file.restore_requested"
		if result.Status == FileRestoreActionRestored {
			eventType = "file.restored"
		}
		s.publishFileProjection(fileID, eventType, nil)
	}
	return result, err
}

func (s *FileSvc) ApproveFileRestore(ownerID, roomCode, requestID string) error {
	var fileID string
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
		fileID = request.FileID
		file, cycle, err := loadCurrentTrash(tx, room.ID, request.FileID)
		if err != nil || cycle.ID != request.TrashCycleID || cycle.Version != request.TrashVersion {
			if err != nil {
				if _, business := errx.As(err); !business {
					return err
				}
			}
			return errx.New(errc.ErrFileRestoreRequestState, nil)
		}
		now := s.now().Unix()
		return restoreTrashCycle(tx, file, cycle, ownerID, &request, now)
	})
	if err == nil {
		s.publishFileProjection(fileID, "file.restored", nil)
	}
	return err
}

func (s *FileSvc) RejectFileRestore(ownerID, roomCode, requestID, reason string) error {
	reason, err := normalizeFileLifecycleReason(reason)
	if err != nil {
		return err
	}
	var fileID string
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
		fileID = request.FileID
		file, cycle, err := loadCurrentTrash(tx, room.ID, request.FileID)
		if err != nil || cycle.ID != request.TrashCycleID || cycle.Version != request.TrashVersion {
			if err != nil {
				if _, business := errx.As(err); !business {
					return err
				}
			}
			return errx.New(errc.ErrFileRestoreRequestState, nil)
		}
		now := s.now().Unix()
		update := tx.Model(&model.FileRestoreRequest{}).
			Where("id = ? AND status = ?", request.ID, model.FileRestorePending).
			Updates(map[string]interface{}{"status": model.FileRestoreRejected, "decided_at": now, "decided_by_user_id": ownerID, "rejection_reason": reason})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errx.New(errc.ErrFileRestoreRequestState, nil)
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, ownerID, FileEventRestoreRejected, now)
	})
	if err == nil {
		s.publishFileProjection(fileID, "file.restore_rejected", nil)
	}
	return err
}

// PurgeFile permanently removes stored bytes and then finalizes the database
// state. A short-lived purging state makes a process interruption recoverable.
func (s *FileSvc) PurgeFile(userID, roomCode, fileID string) error {
	var file model.RoomFile
	var cycle model.FileTrashCycle
	var cancelledTasks []model.DownloadTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, userID)
		if err != nil {
			return err
		}
		file, cycle, err = loadCurrentTrash(tx, room.ID, fileID)
		if err != nil {
			return err
		}
		allowed := member.Role == model.MemberRoleOwner || (file.UploaderUserID == userID && cycle.DeletedByUserID == userID)
		if !allowed {
			return errx.New(errc.ErrFileNotFound, nil)
		}
		update := tx.Model(&model.RoomFile{}).
			Where("id = ? AND status = ? AND trash_version = ?", file.ID, model.FileStatusTrashed, cycle.Version).
			Update("status", model.FileStatusPurging)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		if err := tx.Model(&model.FileTrashCycle{}).
			Where("id = ? AND outcome = ?", cycle.ID, model.FileTrashActive).
			Update("resolved_by_user_id", userID).Error; err != nil {
			return err
		}
		now := s.now().Unix()
		if err := tx.Where("file_id = ? AND status IN ?", file.ID, []string{model.DownloadTaskPending, model.DownloadTaskStreaming}).Find(&cancelledTasks).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.DownloadTask{}).
			Where("file_id = ? AND status IN ?", file.ID, []string{model.DownloadTaskPending, model.DownloadTaskStreaming}).
			Updates(map[string]interface{}{"status": model.DownloadTaskCancelled, "cancelled_at": now}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.downloads.cancelFile(file.ID)
	s.publishCancelledDownloads(roomCode, file.ID, cancelledTasks)
	if s.storage == nil {
		_ = s.rollbackPurge(file.ID, cycle.ID, cycle.Version)
		return errx.New(errc.ErrFilePurge, nil)
	}
	if err := s.storage.DeleteFile(file.RoomID, file.StorageName); err != nil {
		_ = s.rollbackPurge(file.ID, cycle.ID, cycle.Version)
		return errx.Wrap(errc.ErrFilePurge, err, nil)
	}
	if err := s.finalizePurge(file.ID, cycle.Version, userID, s.now().Unix()); err != nil {
		return err
	}
	s.publishFileProjection(file.ID, "file.purged", nil)
	return nil
}

// RecoverPurgingFiles completes idempotent physical deletion after an
// interrupted process. Files that cannot be finalized stay in purging state so
// the service start fails visibly instead of serving missing content.
func (s *FileSvc) RecoverPurgingFiles() error {
	if s == nil || s.db == nil || s.storage == nil {
		return nil
	}
	var files []model.RoomFile
	if err := s.db.Where("status = ?", model.FileStatusPurging).Find(&files).Error; err != nil {
		return err
	}
	for _, file := range files {
		var cycle model.FileTrashCycle
		if err := s.db.Where("file_id = ? AND version = ? AND outcome = ?", file.ID, file.TrashVersion, model.FileTrashActive).First(&cycle).Error; err != nil {
			return err
		}
		if err := s.storage.DeleteFile(file.RoomID, file.StorageName); err != nil {
			return errx.Wrap(errc.ErrFileStorage, err, nil)
		}
		if err := s.finalizePurge(file.ID, cycle.Version, cycle.ResolvedByUserID, s.now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileSvc) finalizePurge(fileID string, version int64, actorID string, now int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND status = ? AND trash_version = ?", fileID, model.FileStatusPurging, version).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		var cycle model.FileTrashCycle
		if err := tx.Where("file_id = ? AND version = ? AND outcome = ?", file.ID, version, model.FileTrashActive).First(&cycle).Error; err != nil {
			return err
		}
		capacity := tx.Model(&model.Room{}).
			Where("id = ? AND used_bytes >= ?", file.RoomID, file.ActualSize).
			Update("used_bytes", gorm.Expr("used_bytes - ?", file.ActualSize))
		if capacity.Error != nil {
			return capacity.Error
		}
		if capacity.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		fileUpdate := tx.Model(&model.RoomFile{}).
			Where("id = ? AND status = ? AND trash_version = ?", file.ID, model.FileStatusPurging, version).
			Update("status", model.FileStatusPurged)
		if fileUpdate.Error != nil {
			return fileUpdate.Error
		}
		if fileUpdate.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		cycleUpdate := tx.Model(&model.FileTrashCycle{}).
			Where("id = ? AND outcome = ?", cycle.ID, model.FileTrashActive).
			Updates(map[string]interface{}{"outcome": model.FileTrashPurged, "resolved_by_user_id": actorID, "resolved_at": now})
		if cycleUpdate.Error != nil {
			return cycleUpdate.Error
		}
		if cycleUpdate.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		if err := tx.Model(&model.FileRestoreRequest{}).
			Where("trash_cycle_id = ? AND status = ?", cycle.ID, model.FileRestorePending).
			Updates(map[string]interface{}{"status": model.FileRestoreInvalidated, "decided_at": now, "decided_by_user_id": actorID}).Error; err != nil {
			return err
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, actorID, FileEventPurged, now)
	})
}

func (s *FileSvc) rollbackPurge(fileID, cycleID string, version int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&model.RoomFile{}).
			Where("id = ? AND status = ? AND trash_version = ?", fileID, model.FileStatusPurging, version).
			Update("status", model.FileStatusTrashed)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		return tx.Model(&model.FileTrashCycle{}).
			Where("id = ? AND outcome = ?", cycleID, model.FileTrashActive).
			Update("resolved_by_user_id", "").Error
	})
}

func (s *FileSvc) publishCancelledDownloads(roomCode, fileID string, tasks []model.DownloadTask) {
	if s == nil || s.hub == nil {
		return
	}
	for _, task := range tasks {
		s.hub.PublishUser(task.UserID, "file.download_cancelled", map[string]interface{}{"roomCode": roomCode, "fileId": fileID, "taskId": task.ID})
	}
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
		return request, errx.New(errc.ErrFileRestoreRequestState, nil)
	}
	update := tx.Model(&model.FileRestoreRequest{}).
		Where("id = ? AND status = ?", request.ID, model.FileRestoreInvalidated).
		Updates(map[string]interface{}{"status": model.FileRestorePending, "created_at": now, "decided_at": nil, "decided_by_user_id": "", "rejection_reason": ""})
	if update.Error != nil {
		return request, update.Error
	}
	if update.RowsAffected != 1 {
		return request, errx.New(errc.ErrFileRestoreRequestState, nil)
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
		return "", errx.New(errc.ErrFileLifecycleReason, nil)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", errx.New(errc.ErrFileLifecycleReason, nil)
		}
	}
	return value, nil
}

func restoreRequestStateError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errx.New(errc.ErrFileRestoreRequestState, nil)
	}
	return err
}
