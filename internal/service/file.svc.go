package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	MaxUploadFiles             = 100
	MaxOriginalNameRunes       = 255
	UploadStartTTL             = 10 * time.Minute
	UploadSmallFileLimit int64 = 10 * 1024 * 1024
	UploadChunkSize      int64 = 5 * 1024 * 1024
	UploadGlobalLimit          = 4
	UploadRoomLimit            = 2
	UploadUserLimit            = 2

	FileEventBatchCreated      = "batch_created"
	FileEventUploadStarted     = "upload_started"
	FileEventUploadCompleted   = "upload_completed"
	FileEventUploadFailed      = "upload_failed"
	FileEventUploadCancelled   = "upload_cancelled"
	FileEventDirectSent        = "direct_sent"
	FileEventReused            = "reused"
	FileEventPublished         = "published_shared"
	FileEventAccepted          = "accepted"
	FileEventDeclined          = "declined"
	FileEventDownloadStarted   = "download_started"
	FileEventDownloaded        = "download_completed"
	FileEventDownloadFailed    = "download_failed"
	FileEventDownloadCancelled = "download_cancelled"
	FileEventTrashed           = "trashed"
	FileEventRestoreRequested  = "restore_requested"
	FileEventRestoreRejected   = "restore_rejected"
	FileEventRestored          = "restored"
	FileEventPurged            = "purged"
)

type FileManifest struct {
	OriginalName string
	DeclaredSize int64
	DeclaredMIME string
}

type fileEventPayload struct {
	RecipientIDs        []string `json:"recipientIds,omitempty"`
	SkippedRecipientIDs []string `json:"skippedRecipientIds,omitempty"`
	RecipientUserID     string   `json:"recipientUserId,omitempty"`
	DeliveryVersion     int64    `json:"deliveryVersion,omitempty"`
	FailureCode         int      `json:"failureCode,omitempty"`
	Reason              string   `json:"reason,omitempty"`
}

type UploadBatchResult struct {
	Batch model.UploadBatch
	Files []model.RoomFile
}

type ProjectionLevel string

const (
	ProjectionHidden    ProjectionLevel = "hidden"
	ProjectionAnonymous ProjectionLevel = "anonymous"
	ProjectionFull      ProjectionLevel = "full"
)

type FileCapabilities struct {
	CanDownload       bool `json:"canDownload"`
	CanAccept         bool `json:"canAccept"`
	CanDecline        bool `json:"canDecline"`
	CanReuse          bool `json:"canReuse"`
	CanPublishShared  bool `json:"canPublishShared"`
	CanTrash          bool `json:"canTrash"`
	CanSetTrashReason bool `json:"canSetTrashReason"`
}

type FileRecipientView struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status,omitempty"`
}

type FileProjection struct {
	Level          ProjectionLevel     `json:"projection"`
	FileID         string              `json:"fileId"`
	DisplayName    string              `json:"displayName"`
	OriginalName   string              `json:"originalName,omitempty"`
	PrivateCode    string              `json:"privateCode,omitempty"`
	Scope          string              `json:"scope"`
	Size           int64               `json:"size"`
	DeclaredMIME   string              `json:"declaredMime,omitempty"`
	DetectedMIME   string              `json:"detectedMime,omitempty"`
	Status         string              `json:"status"`
	Progress       int                 `json:"progress"`
	UploaderUserID string              `json:"uploaderUserId"`
	UploaderName   string              `json:"uploaderName"`
	CreatedAt      int64               `json:"createdAt"`
	CompletedAt    *int64              `json:"completedAt,omitempty"`
	Recipients     []FileRecipientView `json:"recipients,omitempty"`
	Capabilities   FileCapabilities    `json:"capabilities"`
}

type FileSvc struct {
	db            *gorm.DB
	hub           *StreamHub
	storage       *FileStorage
	uploads       *uploadRegistry
	downloads     *downloadRegistry
	notifications *FileNotificationRecorder
	now           func() time.Time
	finalizeMu    sync.Mutex
}

type uploadRegistry struct {
	mu     sync.Mutex
	active map[string]uploadActive
	rooms  map[string]int
	users  map[string]int
	files  map[string]int
}

type uploadActive struct {
	fileID string
	roomID string
	userID string
	cancel context.CancelFunc
}

func NewFileSvc(db *gorm.DB, hub *StreamHub, storages ...*FileStorage) (*FileSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("file database is required")
	}
	var storage *FileStorage
	if len(storages) > 0 {
		storage = storages[0]
	}
	notifications, err := NewFileNotificationRecorder(db)
	if err != nil {
		return nil, err
	}
	return &FileSvc{db: db, hub: hub, storage: storage, uploads: &uploadRegistry{active: make(map[string]uploadActive), rooms: make(map[string]int), users: make(map[string]int), files: make(map[string]int)}, downloads: &downloadRegistry{active: make(map[string]downloadActive), rooms: make(map[string]int), users: make(map[string]int)}, notifications: notifications, now: time.Now}, nil
}

// CreateUploadBatch validates and reserves the complete batch atomically.
func (s *FileSvc) CreateUploadBatch(userID, roomCode, idempotencyKey, scope string, manifests []FileManifest, recipientIDs []string) (UploadBatchResult, error) {
	if s == nil || s.db == nil {
		return UploadBatchResult{}, fmt.Errorf("file service is nil")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 || len(manifests) == 0 || len(manifests) > MaxUploadFiles {
		return UploadBatchResult{}, errx.New(errc.ErrFileManifest, nil)
	}
	total, err := validateManifest(manifests)
	if err != nil {
		return UploadBatchResult{}, err
	}
	if scope != model.FileScopeShared && scope != model.FileScopeDirect {
		return UploadBatchResult{}, errx.New(errc.ErrFileManifest, nil)
	}

	var result UploadBatchResult
	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		room, member, err := activeFileMember(tx, roomCode, userID)
		if err != nil {
			return err
		}
		var existing model.UploadBatch
		if err := tx.Where("room_id = ? AND creator_user_id = ? AND idempotency_key = ?", room.ID, userID, idempotencyKey).First(&existing).Error; err == nil {
			var files []model.RoomFile
			if err := tx.Where("batch_id = ?", existing.ID).Order("created_at ASC, id ASC").Find(&files).Error; err != nil {
				return err
			}
			result = UploadBatchResult{Batch: existing, Files: files}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		recipients, err := validateRecipients(tx, room.ID, member.UserID, scope, recipientIDs)
		if err != nil {
			return err
		}
		reserve := tx.Model(&model.Room{}).
			Where("id = ? AND status = ? AND capacity_bytes - used_bytes - reserved_bytes >= ?", room.ID, model.RoomStatusActive, total).
			UpdateColumn("reserved_bytes", gorm.Expr("reserved_bytes + ?", total))
		if reserve.Error != nil {
			return reserve.Error
		}
		if reserve.RowsAffected != 1 {
			return errx.New(errc.ErrRoomCapacity, nil)
		}

		now := s.now().Unix()
		batchID, err := ulidx.New()
		if err != nil {
			return err
		}
		batch := model.UploadBatch{ID: batchID, RoomID: room.ID, CreatorUserID: userID, IdempotencyKey: idempotencyKey, Scope: scope, FileCount: len(manifests), DeclaredTotalSize: total, Status: model.UploadBatchPending, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		files := make([]model.RoomFile, 0, len(manifests))
		for _, manifest := range manifests {
			fileID, err := ulidx.New()
			if err != nil {
				return err
			}
			storageName, err := ulidx.New()
			if err != nil {
				return err
			}
			privateCode := ""
			if scope == model.FileScopeDirect {
				privateCode, err = uniquePrivateCode(tx, room.ID)
				if err != nil {
					return err
				}
			}
			file := model.RoomFile{ID: fileID, RoomID: room.ID, BatchID: batchID, UploaderUserID: userID, StorageName: storageName, OriginalName: strings.TrimSpace(manifest.OriginalName), DeclaredMIME: strings.TrimSpace(manifest.DeclaredMIME), DeclaredSize: manifest.DeclaredSize, Scope: scope, PrivateCode: privateCode, Status: model.FileStatusReserved, CreatedAt: now}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
			chunkSize, totalParts := uploadChunkPlan(file.DeclaredSize)
			if err := tx.Create(&model.UploadSession{FileID: file.ID, RoomID: room.ID, UploaderUserID: userID, DeclaredSize: file.DeclaredSize, ChunkSize: chunkSize, TotalParts: totalParts, Status: model.UploadSessionActive, ExpiresAt: room.ExpiresAt, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
			recipientIDs := make([]string, 0, len(recipients))
			for _, recipient := range recipients {
				relationID, err := ulidx.New()
				if err != nil {
					return err
				}
				if err := tx.Create(&model.FileRecipient{ID: relationID, FileID: fileID, RecipientUserID: recipient.UserID, OperationID: fileID, DeliveryVersion: 1, Status: model.RecipientPending, SentAt: now}).Error; err != nil {
					return err
				}
				recipientIDs = append(recipientIDs, recipient.UserID)
			}
			if err := createFileEventWithOperation(tx, room.ID, file.ID, batchID, userID, FileEventBatchCreated, file.ID, now, nil); err != nil {
				return err
			}
			if scope == model.FileScopeDirect {
				if _, err := createFileEventRecordWithOperation(tx, room.ID, file.ID, batchID, userID, FileEventDirectSent, file.ID, now, fileEventPayload{RecipientIDs: recipientIDs}); err != nil {
					return err
				}
			}
			files = append(files, file)
		}
		eventID, err := ulidx.New()
		if err != nil {
			return err
		}
		if err := tx.Create(&model.FileEvent{ID: eventID, RoomID: room.ID, BatchID: batchID, ActorUserID: userID, OperationID: batchID, Type: FileEventBatchCreated, PayloadJSON: "{}", CreatedAt: now}).Error; err != nil {
			return err
		}
		result = UploadBatchResult{Batch: batch, Files: files}
		created = true
		return assertRoomCapacity(tx, room.ID)
	})
	if err == nil && created {
		for _, file := range result.Files {
			s.publishFileProjection(file.ID, "file.created", nil)
		}
	}
	return result, err
}

func (s *FileSvc) MarkUploading(userID, fileID string) error {
	now := s.now().Unix()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND uploader_user_id = ? AND status = ?", fileID, userID, model.FileStatusReserved).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		result := tx.Model(&model.RoomFile{}).Where("id = ? AND status = ?", fileID, model.FileStatusReserved).Updates(map[string]interface{}{"status": model.FileStatusUploading, "started_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventUploadStarted, now)
	})
}

func (s *FileSvc) UploadContent(ctx context.Context, userID, roomCode, fileID string, contentLength int64, source interface{ Read([]byte) (int, error) }) error {
	if s == nil || s.db == nil || s.storage == nil || source == nil {
		return fmt.Errorf("file upload service is unavailable")
	}
	_, member, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return err
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND uploader_user_id = ?", fileID, member.RoomID, userID).First(&file).Error; err != nil {
		return fileNotFound(err)
	}
	if file.Status != model.FileStatusReserved {
		return errx.New(errc.ErrFileState, nil)
	}
	if contentLength <= 0 || contentLength != file.DeclaredSize {
		return errx.New(errc.ErrUploadSize, nil)
	}
	uploadContext, finish, err := s.uploads.begin(ctx, file.ID, file.RoomID, userID)
	if err != nil {
		return err
	}
	defer finish()
	if err := s.MarkUploading(userID, file.ID); err != nil {
		return err
	}

	lastProgress := 0
	lastPublished := time.Time{}
	stored, writeErr := s.storage.Write(uploadContext, file.RoomID, file.StorageName, file.DeclaredSize, source, func(written int64) {
		progress := int(written * 100 / file.DeclaredSize)
		now := s.now()
		if progress <= lastProgress || (progress < 100 && progress-lastProgress < 1) || (progress < 100 && now.Sub(lastPublished) < 500*time.Millisecond) {
			return
		}
		lastProgress = progress
		lastPublished = now
		_ = s.db.Model(&model.RoomFile{}).Where("id = ? AND status = ?", file.ID, model.FileStatusUploading).Update("progress", progress).Error
		s.publishFileProjection(file.ID, "file.upload_progress", map[string]interface{}{"progress": progress})
	})
	if writeErr != nil {
		status := model.FileStatusFailed
		code := errc.ErrFileStorage
		if errors.Is(writeErr, context.Canceled) {
			status = model.FileStatusCancelled
			code = errc.ErrUploadCancelled
		} else if errors.Is(writeErr, ErrStoredSizeMismatch) {
			code = errc.ErrUploadSize
		}
		_ = s.ReleaseUpload(userID, file.ID, status, code)
		s.publishFileProjection(file.ID, "file.upload_failed", map[string]interface{}{"failureCode": code})
		return errx.Wrap(code, writeErr, nil)
	}
	if err := s.CompleteUpload(userID, file.ID, stored.DetectedMIME, stored.Size); err != nil {
		_ = s.storage.DeleteFile(file.RoomID, file.StorageName)
		_ = s.ReleaseUpload(userID, file.ID, model.FileStatusFailed, errc.ErrFileStorage)
		return err
	}
	s.refreshBatchStatus(file.BatchID)
	s.publishFileProjection(file.ID, "file.available", map[string]interface{}{"progress": 100})
	return nil
}

func (s *FileSvc) CancelUpload(userID, roomCode, fileID string) error {
	_, member, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return err
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND uploader_user_id = ?", fileID, member.RoomID, userID).First(&file).Error; err != nil {
		return fileNotFound(err)
	}
	s.uploads.cancel(file.ID)
	transitioned := false
	switch file.Status {
	case model.FileStatusReserved, model.FileStatusUploading:
		transitioned, err = s.releaseUpload(userID, file.ID, model.FileStatusCancelled, errc.ErrUploadCancelled)
		if err != nil {
			return err
		}
	case model.FileStatusCancelled, model.FileStatusFailed:
		// A previous request may have completed the state transition while its
		// response was lost. Continue cleaning artifacts so retry is idempotent.
	default:
		return errx.New(errc.ErrFileState, nil)
	}
	if err := s.db.Where("file_id = ?", file.ID).Delete(&model.UploadPart{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("file_id = ?", file.ID).Delete(&model.UploadSession{}).Error; err != nil {
		return err
	}
	if s.storage != nil {
		if err := s.storage.DeleteTemporaryFile(file.RoomID, file.StorageName); err != nil {
			return errx.Wrap(errc.ErrFileStorage, err, nil)
		}
	}
	s.refreshBatchStatus(file.BatchID)
	if transitioned {
		s.publishFileProjection(file.ID, "file.upload_cancelled", map[string]interface{}{"failureCode": errc.ErrUploadCancelled})
	}
	return nil
}

func (s *FileSvc) CompleteUpload(userID, fileID, detectedMIME string, actualSize int64) error {
	now := s.now().Unix()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND uploader_user_id = ?", fileID, userID).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		if file.Status != model.FileStatusUploading || actualSize != file.DeclaredSize {
			if actualSize != file.DeclaredSize {
				return errx.New(errc.ErrUploadSize, nil)
			}
			return errx.New(errc.ErrFileState, nil)
		}
		roomResult := tx.Model(&model.Room{}).Where("id = ? AND reserved_bytes >= ? AND used_bytes + ? <= capacity_bytes", file.RoomID, file.DeclaredSize, actualSize).Updates(map[string]interface{}{"reserved_bytes": gorm.Expr("reserved_bytes - ?", file.DeclaredSize), "used_bytes": gorm.Expr("used_bytes + ?", actualSize)})
		if roomResult.Error != nil {
			return roomResult.Error
		}
		if roomResult.RowsAffected != 1 {
			return errx.New(errc.ErrRoomCapacity, nil)
		}
		fileResult := tx.Model(&model.RoomFile{}).Where("id = ? AND status = ?", file.ID, model.FileStatusUploading).Updates(map[string]interface{}{"status": model.FileStatusAvailable, "actual_size": actualSize, "detected_mime": detectedMIME, "progress": 100, "completed_at": now})
		if fileResult.Error != nil {
			return fileResult.Error
		}
		if fileResult.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		if err := tx.Model(&model.UploadSession{}).Where("file_id = ? AND status = ?", file.ID, model.UploadSessionActive).Updates(map[string]interface{}{"status": model.UploadSessionCompleted, "received_bytes": actualSize, "updated_at": now, "completed_at": now}).Error; err != nil {
			return err
		}
		event, err := createFileEventRecord(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventUploadCompleted, now)
		if err != nil {
			return err
		}
		if file.Scope == model.FileScopeDirect {
			var recipients []model.FileRecipient
			if err := tx.Where("file_id = ? AND status = ?", file.ID, model.RecipientPending).Find(&recipients).Error; err != nil {
				return err
			}
			for _, recipient := range recipients {
				if err := s.recordFileNotification(tx, NotificationTypeFileReceived, recipient.RecipientUserID, file.UploaderUserID, file, recipient, event); err != nil {
					return err
				}
			}
		}
		return assertRoomCapacity(tx, file.RoomID)
	})
}

func (s *FileSvc) ReleaseUpload(userID, fileID, targetStatus string, failureCode int) error {
	_, err := s.releaseUpload(userID, fileID, targetStatus, failureCode)
	return err
}

func (s *FileSvc) releaseUpload(userID, fileID, targetStatus string, failureCode int) (bool, error) {
	if targetStatus != model.FileStatusFailed && targetStatus != model.FileStatusCancelled {
		return false, errx.New(errc.ErrFileState, nil)
	}
	now := s.now().Unix()
	transitioned := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND uploader_user_id = ?", fileID, userID).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		if file.Status == targetStatus {
			return nil
		}
		if file.Status != model.FileStatusReserved && file.Status != model.FileStatusUploading {
			return errx.New(errc.ErrFileState, nil)
		}
		result := tx.Model(&model.RoomFile{}).Where("id = ? AND status IN ?", file.ID, []string{model.FileStatusReserved, model.FileStatusUploading}).Updates(map[string]interface{}{"status": targetStatus, "failure_code": failureCode, "failed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		roomResult := tx.Model(&model.Room{}).Where("id = ? AND reserved_bytes >= ?", file.RoomID, file.DeclaredSize).UpdateColumn("reserved_bytes", gorm.Expr("reserved_bytes - ?", file.DeclaredSize))
		if roomResult.Error != nil {
			return roomResult.Error
		}
		if roomResult.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		sessionStatus := model.UploadSessionFailed
		if targetStatus == model.FileStatusCancelled {
			sessionStatus = model.UploadSessionCancelled
		}
		if err := tx.Model(&model.UploadSession{}).Where("file_id = ? AND status = ?", file.ID, model.UploadSessionActive).Updates(map[string]interface{}{"status": sessionStatus, "updated_at": now}).Error; err != nil {
			return err
		}
		eventType := FileEventUploadFailed
		if targetStatus == model.FileStatusCancelled {
			eventType = FileEventUploadCancelled
		}
		if err := createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, eventType, now); err != nil {
			return err
		}
		transitioned = true
		return assertRoomCapacity(tx, file.RoomID)
	})
	return transitioned, err
}

func (s *FileSvc) refreshBatchStatus(batchID string) {
	if batchID == "" {
		return
	}
	var files []model.RoomFile
	if err := s.db.Select("status").Where("batch_id = ?", batchID).Find(&files).Error; err != nil || len(files) == 0 {
		return
	}
	available, active, terminalFailure := 0, 0, 0
	for _, file := range files {
		switch file.Status {
		case model.FileStatusAvailable:
			available++
		case model.FileStatusReserved, model.FileStatusUploading:
			active++
		case model.FileStatusFailed, model.FileStatusCancelled:
			terminalFailure++
		}
	}
	status := model.UploadBatchUploading
	if active == 0 {
		switch {
		case available == len(files):
			status = model.UploadBatchCompleted
		case terminalFailure == len(files):
			status = model.UploadBatchFailed
		default:
			status = model.UploadBatchPartial
		}
	}
	_ = s.db.Model(&model.UploadBatch{}).Where("id = ?", batchID).Updates(map[string]interface{}{"status": status, "updated_at": s.now().Unix()}).Error
}

func (r *uploadRegistry) begin(parent context.Context, fileID, roomID, userID string) (context.Context, func(), error) {
	return r.beginPart(parent, fileID, fileID, roomID, userID)
}

func (r *uploadRegistry) beginPart(parent context.Context, key, fileID, roomID, userID string) (context.Context, func(), error) {
	if parent == nil {
		parent = context.Background()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.files == nil {
		r.files = make(map[string]int)
	}
	if _, exists := r.active[key]; exists || len(r.active) >= UploadGlobalLimit || r.rooms[roomID] >= UploadRoomLimit || r.users[userID] >= UploadUserLimit || r.files[fileID] >= 2 {
		return nil, nil, errx.New(errc.ErrUploadLimited, nil)
	}
	ctx, cancel := context.WithCancel(parent)
	r.active[key] = uploadActive{fileID: fileID, roomID: roomID, userID: userID, cancel: cancel}
	r.rooms[roomID]++
	r.users[userID]++
	r.files[fileID]++
	var once sync.Once
	finish := func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.active, key)
			r.rooms[roomID]--
			r.users[userID]--
			r.files[fileID]--
			r.mu.Unlock()
			cancel()
		})
	}
	return ctx, finish, nil
}

func (r *uploadRegistry) cancel(fileID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0)
	for _, active := range r.active {
		if active.fileID == fileID {
			cancels = append(cancels, active.cancel)
		}
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (r *uploadRegistry) cancelMatching(roomID, userID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0)
	for _, active := range r.active {
		if active.roomID == roomID && (userID == "" || active.userID == userID) {
			cancels = append(cancels, active.cancel)
		}
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *FileSvc) CancelMemberTransfers(roomID, userID string) {
	if s == nil {
		return
	}
	s.uploads.cancelMatching(roomID, userID)
	s.downloads.cancelMatching(roomID, userID)
}

func (s *FileSvc) CancelRoomTransfers(roomID string) {
	if s == nil {
		return
	}
	s.uploads.cancelMatching(roomID, "")
	s.downloads.cancelMatching(roomID, "")
}

func ProjectFile(file model.RoomFile, uploader model.RoomMember, recipients []model.FileRecipient, recipientMembers map[string]model.RoomMember, viewer model.RoomMember) FileProjection {
	level := ProjectionHidden
	participant := file.UploaderUserID == viewer.UserID
	viewerStatus := ""
	for _, recipient := range recipients {
		if recipient.RecipientUserID == viewer.UserID {
			participant = true
			viewerStatus = recipient.Status
			break
		}
	}
	if file.Scope == model.FileScopeShared || participant {
		level = ProjectionFull
	} else if viewer.Role == model.MemberRoleOwner {
		level = ProjectionAnonymous
	}
	if level == ProjectionHidden {
		return FileProjection{Level: ProjectionHidden}
	}
	name := file.OriginalName
	if level == ProjectionAnonymous {
		name = file.PrivateCode
	}
	projection := FileProjection{Level: level, FileID: file.ID, DisplayName: name, PrivateCode: file.PrivateCode, Scope: file.Scope, Size: file.DeclaredSize, Status: file.Status, Progress: file.Progress, UploaderUserID: file.UploaderUserID, UploaderName: uploader.DisplayName, CreatedAt: file.CreatedAt, CompletedAt: file.CompletedAt}
	projection.Capabilities.CanTrash = file.Status == model.FileStatusAvailable && (viewer.Role == model.MemberRoleOwner || file.UploaderUserID == viewer.UserID)
	projection.Capabilities.CanSetTrashReason = projection.Capabilities.CanTrash && viewer.Role == model.MemberRoleOwner && file.UploaderUserID != viewer.UserID
	for _, recipient := range recipients {
		member := recipientMembers[recipient.RecipientUserID]
		view := FileRecipientView{UserID: recipient.RecipientUserID, DisplayName: member.DisplayName}
		if level == ProjectionFull && (file.UploaderUserID == viewer.UserID || recipient.RecipientUserID == viewer.UserID) {
			view.Status = recipient.Status
		}
		projection.Recipients = append(projection.Recipients, view)
	}
	if level == ProjectionFull {
		projection.OriginalName = file.OriginalName
		projection.DeclaredMIME = file.DeclaredMIME
		projection.DetectedMIME = file.DetectedMIME
		projection.Capabilities.CanReuse = file.Scope == model.FileScopeDirect && file.Status == model.FileStatusAvailable && file.UploaderUserID == viewer.UserID
		projection.Capabilities.CanPublishShared = projection.Capabilities.CanReuse
		projection.Capabilities.CanAccept = file.Scope == model.FileScopeDirect && file.Status == model.FileStatusAvailable && viewerStatus == model.RecipientPending
		projection.Capabilities.CanDecline = projection.Capabilities.CanAccept
		projection.Capabilities.CanDownload = file.Status == model.FileStatusAvailable && (file.Scope == model.FileScopeShared || file.UploaderUserID == viewer.UserID || viewerStatus == model.RecipientAccepted || viewerStatus == model.RecipientDownloaded)
	}
	return projection
}

func validateManifest(manifests []FileManifest) (int64, error) {
	var total int64
	for _, manifest := range manifests {
		name := strings.TrimSpace(manifest.OriginalName)
		if name == "" || len([]rune(name)) > MaxOriginalNameRunes || manifest.DeclaredSize <= 0 {
			return 0, errx.New(errc.ErrFileManifest, nil)
		}
		for _, r := range name {
			if unicode.IsControl(r) {
				return 0, errx.New(errc.ErrFileManifest, nil)
			}
		}
		if manifest.DeclaredSize > math.MaxInt64-total {
			return 0, errx.New(errc.ErrFileManifest, nil)
		}
		total += manifest.DeclaredSize
	}
	return total, nil
}

func validateRecipients(tx *gorm.DB, roomID, uploaderID, scope string, recipientIDs []string) ([]model.RoomMember, error) {
	if scope == model.FileScopeShared {
		if len(recipientIDs) != 0 {
			return nil, errx.New(errc.ErrFileRecipient, nil)
		}
		return nil, nil
	}
	unique := make(map[string]struct{}, len(recipientIDs))
	for _, id := range recipientIDs {
		id = strings.TrimSpace(id)
		if id == "" || id == uploaderID {
			return nil, errx.New(errc.ErrFileRecipient, nil)
		}
		unique[id] = struct{}{}
	}
	if len(unique) == 0 || len(unique) != len(recipientIDs) {
		return nil, errx.New(errc.ErrFileRecipient, nil)
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	var members []model.RoomMember
	if err := tx.Where("room_id = ? AND user_id IN ? AND status = ?", roomID, ids, model.MemberStatusActive).Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) != len(ids) {
		return nil, errx.New(errc.ErrFileRecipient, nil)
	}
	return members, nil
}

func activeFileMember(tx *gorm.DB, roomCode, userID string) (model.Room, model.RoomMember, error) {
	var room model.Room
	if err := tx.Where("code = ? AND status = ?", strings.TrimSpace(roomCode), model.RoomStatusActive).Order("created_at DESC").First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return room, model.RoomMember{}, errx.New(errc.ErrRoomNotFound, nil)
		}
		return room, model.RoomMember{}, err
	}
	var member model.RoomMember
	if err := tx.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return room, member, errx.New(errc.ErrMemberRequired, nil)
		}
		return room, member, err
	}
	return room, member, nil
}

func uniquePrivateCode(tx *gorm.DB, roomID string) (string, error) {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	for attempt := 0; attempt < 20; attempt++ {
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		for index := range bytes {
			bytes[index] = alphabet[int(bytes[index])%len(alphabet)]
		}
		code := string(bytes[:4]) + "-" + string(bytes[4:])
		var count int64
		if err := tx.Model(&model.RoomFile{}).Where("room_id = ? AND private_code = ?", roomID, code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("generate unique private file code")
}

func assertRoomCapacity(tx *gorm.DB, roomID string) error {
	var room model.Room
	if err := tx.Select("id", "capacity_bytes", "used_bytes", "reserved_bytes").Where("id = ?", roomID).First(&room).Error; err != nil {
		return err
	}
	if room.CapacityBytes < 0 || room.UsedBytes < 0 || room.ReservedBytes < 0 || room.UsedBytes > room.CapacityBytes-room.ReservedBytes {
		return fmt.Errorf("room capacity invariant violated")
	}
	return nil
}

func createFileEvent(tx *gorm.DB, roomID, fileID, batchID, actorID, eventType string, now int64) error {
	_, err := createFileEventRecord(tx, roomID, fileID, batchID, actorID, eventType, now)
	return err
}

func createFileEventWithOperation(tx *gorm.DB, roomID, fileID, batchID, actorID, eventType, operationID string, now int64, payload interface{}) error {
	_, err := createFileEventRecordWithOperation(tx, roomID, fileID, batchID, actorID, eventType, operationID, now, payload)
	return err
}

func createFileEventRecord(tx *gorm.DB, roomID, fileID, batchID, actorID, eventType string, now int64) (model.FileEvent, error) {
	return createFileEventRecordWithOperation(tx, roomID, fileID, batchID, actorID, eventType, fileID, now, nil)
}

func createFileEventRecordWithOperation(tx *gorm.DB, roomID, fileID, batchID, actorID, eventType, operationID string, now int64, payload interface{}) (model.FileEvent, error) {
	id, err := ulidx.New()
	if err != nil {
		return model.FileEvent{}, err
	}
	if operationID == "" {
		operationID = fileID
	}
	if operationID == "" {
		operationID = batchID
	}
	payloadJSON := "{}"
	if payload != nil {
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return model.FileEvent{}, encodeErr
		}
		payloadJSON = string(encoded)
	}
	event := model.FileEvent{ID: id, RoomID: roomID, FileID: fileID, BatchID: batchID, ActorUserID: actorID, OperationID: operationID, Type: eventType, PayloadJSON: payloadJSON, CreatedAt: now}
	return event, tx.Create(&event).Error
}

func (s *FileSvc) recordFileNotification(tx *gorm.DB, notificationType, userID, counterpartUserID string, file model.RoomFile, recipient model.FileRecipient, event model.FileEvent) error {
	if s == nil || s.notifications == nil {
		return fmt.Errorf("file notification recorder is unavailable")
	}
	_, err := s.notifications.Record(tx, FileNotificationRecordInput{
		UserID:            userID,
		Type:              notificationType,
		RoomID:            file.RoomID,
		FileID:            file.ID,
		CounterpartUserID: counterpartUserID,
		FileRecipientID:   recipient.ID,
		DeliveryVersion:   recipient.DeliveryVersion,
		SourceEventID:     event.ID,
		OccurredAtMS:      event.CreatedAt * 1000,
	})
	return err
}

func (s *FileSvc) recordTrashNotification(tx *gorm.DB, notificationType, userID, counterpartUserID string, file model.RoomFile, cycle model.FileTrashCycle, request *model.FileRestoreRequest, event model.FileEvent) error {
	if s == nil || s.notifications == nil {
		return fmt.Errorf("file notification recorder is unavailable")
	}
	input := FileNotificationRecordInput{
		UserID:            userID,
		Type:              notificationType,
		RoomID:            file.RoomID,
		FileID:            file.ID,
		CounterpartUserID: counterpartUserID,
		TrashCycleID:      cycle.ID,
		TrashVersion:      cycle.Version,
		SourceEventID:     event.ID,
		OccurredAtMS:      event.CreatedAt * 1000,
	}
	if request != nil {
		input.RestoreRequestID = request.ID
	}
	_, err := s.notifications.Record(tx, input)
	return err
}

func (s *FileSvc) publishNotificationChanged(userIDs []string, reason string) {
	if s == nil || s.hub == nil {
		return
	}
	s.hub.PublishUsers(userIDs, "notification.changed", map[string]interface{}{"reason": reason})
}

func (s *FileSvc) fileNotificationAudience(fileID string, userIDs ...string) []string {
	unique := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID != "" {
			unique[userID] = struct{}{}
		}
	}
	if s != nil && s.db != nil && fileID != "" {
		var file model.RoomFile
		if err := s.db.Select("uploader_user_id").Where("id = ?", fileID).First(&file).Error; err == nil && file.UploaderUserID != "" {
			unique[file.UploaderUserID] = struct{}{}
		}
		var recipientIDs []string
		if err := s.db.Model(&model.FileRecipient{}).Where("file_id = ?", fileID).Pluck("recipient_user_id", &recipientIDs).Error; err == nil {
			for _, userID := range recipientIDs {
				if userID != "" {
					unique[userID] = struct{}{}
				}
			}
		}
	}
	result := make([]string, 0, len(unique))
	for userID := range unique {
		result = append(result, userID)
	}
	return result
}

func fileNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errx.New(errc.ErrFileNotFound, nil)
	}
	return err
}
