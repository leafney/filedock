package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	MaxUploadFiles       = 100
	MaxOriginalNameRunes = 255
	UploadStartTTL       = 10 * time.Minute

	FileEventBatchCreated    = "batch_created"
	FileEventUploadStarted   = "upload_started"
	FileEventUploadCompleted = "upload_completed"
	FileEventUploadFailed    = "upload_failed"
	FileEventUploadCancelled = "upload_cancelled"
	FileEventDirectSent      = "direct_sent"
	FileEventReused          = "reused"
	FileEventPublished       = "published_shared"
	FileEventAccepted        = "accepted"
	FileEventDeclined        = "declined"
	FileEventDownloadStarted = "download_started"
	FileEventDownloaded      = "download_completed"
)

type FileManifest struct {
	OriginalName string
	DeclaredSize int64
	DeclaredMIME string
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
	CanDownload      bool `json:"canDownload"`
	CanAccept        bool `json:"canAccept"`
	CanDecline       bool `json:"canDecline"`
	CanReuse         bool `json:"canReuse"`
	CanPublishShared bool `json:"canPublishShared"`
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
	db  *gorm.DB
	hub *StreamHub
	now func() time.Time
}

func NewFileSvc(db *gorm.DB, hub *StreamHub) (*FileSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("file database is required")
	}
	return &FileSvc{db: db, hub: hub, now: time.Now}, nil
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
			for _, recipient := range recipients {
				relationID, err := ulidx.New()
				if err != nil {
					return err
				}
				if err := tx.Create(&model.FileRecipient{ID: relationID, FileID: fileID, RecipientUserID: recipient.UserID, Status: model.RecipientPending, SentAt: now}).Error; err != nil {
					return err
				}
			}
			files = append(files, file)
		}
		eventID, err := ulidx.New()
		if err != nil {
			return err
		}
		if err := tx.Create(&model.FileEvent{ID: eventID, RoomID: room.ID, BatchID: batchID, ActorUserID: userID, Type: FileEventBatchCreated, CreatedAt: now}).Error; err != nil {
			return err
		}
		result = UploadBatchResult{Batch: batch, Files: files}
		return assertRoomCapacity(tx, room.ID)
	})
	return result, err
}

func (s *FileSvc) MarkUploading(userID, fileID string) error {
	now := s.now().Unix()
	result := s.db.Model(&model.RoomFile{}).Where("id = ? AND uploader_user_id = ? AND status = ?", fileID, userID, model.FileStatusReserved).Updates(map[string]interface{}{"status": model.FileStatusUploading, "started_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errx.New(errc.ErrFileState, nil)
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
		if err := createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventUploadCompleted, now); err != nil {
			return err
		}
		return assertRoomCapacity(tx, file.RoomID)
	})
}

func (s *FileSvc) ReleaseUpload(userID, fileID, targetStatus string, failureCode int) error {
	if targetStatus != model.FileStatusFailed && targetStatus != model.FileStatusCancelled {
		return errx.New(errc.ErrFileState, nil)
	}
	now := s.now().Unix()
	return s.db.Transaction(func(tx *gorm.DB) error {
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
		roomResult := tx.Model(&model.Room{}).Where("id = ? AND reserved_bytes >= ?", file.RoomID, file.DeclaredSize).UpdateColumn("reserved_bytes", gorm.Expr("reserved_bytes - ?", file.DeclaredSize))
		if roomResult.Error != nil {
			return roomResult.Error
		}
		if roomResult.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		if err := tx.Model(&model.RoomFile{}).Where("id = ? AND status IN ?", file.ID, []string{model.FileStatusReserved, model.FileStatusUploading}).Updates(map[string]interface{}{"status": targetStatus, "failure_code": failureCode, "failed_at": now}).Error; err != nil {
			return err
		}
		eventType := FileEventUploadFailed
		if targetStatus == model.FileStatusCancelled {
			eventType = FileEventUploadCancelled
		}
		if err := createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, eventType, now); err != nil {
			return err
		}
		return assertRoomCapacity(tx, file.RoomID)
	})
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
		name = "private#" + file.PrivateCode
	}
	projection := FileProjection{Level: level, FileID: file.ID, DisplayName: name, PrivateCode: file.PrivateCode, Scope: file.Scope, Size: file.DeclaredSize, Status: file.Status, Progress: file.Progress, UploaderUserID: file.UploaderUserID, UploaderName: uploader.DisplayName, CreatedAt: file.CreatedAt, CompletedAt: file.CompletedAt}
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
	id, err := ulidx.New()
	if err != nil {
		return err
	}
	return tx.Create(&model.FileEvent{ID: id, RoomID: roomID, FileID: fileID, BatchID: batchID, ActorUserID: actorID, Type: eventType, CreatedAt: now}).Error
}

func fileNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errx.New(errc.ErrFileNotFound, nil)
	}
	return err
}
