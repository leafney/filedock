package service

import (
	"fmt"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	NotificationTypeFileReceived         = "file_received"
	NotificationTypeFileDeclined         = "file_declined"
	NotificationTypeFileDownloaded       = "file_downloaded"
	NotificationTypeFileTrashedByOwner   = "file_trashed_by_owner"
	NotificationTypeFileRestoreRequested = "file_restore_requested"
	NotificationTypeFileRestoredByOwner  = "file_restored_by_owner"
	NotificationTypeFileRestoreRejected  = "file_restore_rejected"
	NotificationTypeFilePurgedByOwner    = "file_purged_by_owner"
)

type FileNotificationRecordInput struct {
	UserID            string
	Type              string
	RoomID            string
	FileID            string
	CounterpartUserID string
	OccurrenceKey     string
	FileRecipientID   string
	DeliveryVersion   int64
	TrashCycleID      string
	TrashVersion      int64
	RestoreRequestID  string
	SourceEventID     string
	OccurredAtMS      int64
}

// FileNotificationRecorder owns durable file-notification occurrence writes.
// Callers pass their active transaction so business state and its notification
// either commit together or both roll back.
type FileNotificationRecorder struct {
	db *gorm.DB
}

func NewFileNotificationRecorder(db *gorm.DB) (*FileNotificationRecorder, error) {
	if db == nil {
		return nil, fmt.Errorf("notification record database is required")
	}
	return &FileNotificationRecorder{db: db}, nil
}

func (r *FileNotificationRecorder) Record(tx *gorm.DB, input FileNotificationRecordInput) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("notification recorder is not initialized")
	}
	if tx == nil {
		tx = r.db
	}
	if input.OccurrenceKey == "" {
		input.OccurrenceKey = fileNotificationOccurrenceKey(input)
	}
	if !validFileNotificationInput(input) {
		return false, fmt.Errorf("file notification record is invalid")
	}
	id, err := ulidx.New()
	if err != nil {
		return false, err
	}
	record := model.NotificationRecord{
		ID:                id,
		UserID:            input.UserID,
		Type:              input.Type,
		OccurrenceKey:     input.OccurrenceKey,
		RoomID:            input.RoomID,
		FileID:            input.FileID,
		CounterpartUserID: input.CounterpartUserID,
		FileRecipientID:   input.FileRecipientID,
		DeliveryVersion:   input.DeliveryVersion,
		TrashCycleID:      input.TrashCycleID,
		TrashVersion:      input.TrashVersion,
		RestoreRequestID:  input.RestoreRequestID,
		SourceEventID:     input.SourceEventID,
		OccurredAtMS:      input.OccurredAtMS,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "type"},
			{Name: "occurrence_key"},
		},
		DoNothing: true,
	}).Create(&record)
	return result.RowsAffected == 1, result.Error
}

func isFileNotificationType(value string) bool {
	switch value {
	case NotificationTypeFileReceived,
		NotificationTypeFileDeclined,
		NotificationTypeFileDownloaded,
		NotificationTypeFileTrashedByOwner,
		NotificationTypeFileRestoreRequested,
		NotificationTypeFileRestoredByOwner,
		NotificationTypeFileRestoreRejected,
		NotificationTypeFilePurgedByOwner:
		return true
	default:
		return false
	}
}

func isRecipientFileNotificationType(value string) bool {
	return value == NotificationTypeFileReceived || value == NotificationTypeFileDeclined || value == NotificationTypeFileDownloaded
}

func validFileNotificationInput(input FileNotificationRecordInput) bool {
	if !isFileNotificationType(input.Type) || input.UserID == "" || input.RoomID == "" || input.FileID == "" || input.CounterpartUserID == "" || input.OccurrenceKey == "" || input.SourceEventID == "" || input.OccurredAtMS < 1 {
		return false
	}
	if isRecipientFileNotificationType(input.Type) {
		return input.FileRecipientID != "" && input.DeliveryVersion > 0
	}
	if input.TrashCycleID == "" || input.TrashVersion < 1 {
		return false
	}
	if input.Type == NotificationTypeFileRestoreRequested || input.Type == NotificationTypeFileRestoreRejected {
		return input.RestoreRequestID != ""
	}
	return true
}

func fileNotificationOccurrenceKey(input FileNotificationRecordInput) string {
	if isRecipientFileNotificationType(input.Type) && input.FileRecipientID != "" && input.DeliveryVersion > 0 {
		return fmt.Sprintf("recipient:%s:%d", input.FileRecipientID, input.DeliveryVersion)
	}
	if (input.Type == NotificationTypeFileRestoreRequested || input.Type == NotificationTypeFileRestoreRejected) && input.RestoreRequestID != "" {
		return "request:" + input.RestoreRequestID
	}
	if input.TrashCycleID != "" && input.TrashVersion > 0 {
		return fmt.Sprintf("trash:%s:%d", input.TrashCycleID, input.TrashVersion)
	}
	return ""
}
