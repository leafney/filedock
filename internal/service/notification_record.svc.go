package service

import (
	"fmt"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	NotificationTypeFileReceived   = "file_received"
	NotificationTypeFileDeclined   = "file_declined"
	NotificationTypeFileDownloaded = "file_downloaded"
)

type FileNotificationRecordInput struct {
	UserID            string
	Type              string
	RoomID            string
	FileID            string
	CounterpartUserID string
	FileRecipientID   string
	DeliveryVersion   int64
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
	if !isFileNotificationType(input.Type) || input.UserID == "" || input.RoomID == "" || input.FileID == "" || input.CounterpartUserID == "" || input.FileRecipientID == "" || input.DeliveryVersion < 1 || input.SourceEventID == "" || input.OccurredAtMS < 1 {
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
		RoomID:            input.RoomID,
		FileID:            input.FileID,
		CounterpartUserID: input.CounterpartUserID,
		FileRecipientID:   input.FileRecipientID,
		DeliveryVersion:   input.DeliveryVersion,
		SourceEventID:     input.SourceEventID,
		OccurredAtMS:      input.OccurredAtMS,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "type"},
			{Name: "file_recipient_id"},
			{Name: "delivery_version"},
		},
		DoNothing: true,
	}).Create(&record)
	return result.RowsAffected == 1, result.Error
}

func isFileNotificationType(value string) bool {
	return value == NotificationTypeFileReceived || value == NotificationTypeFileDeclined || value == NotificationTypeFileDownloaded
}
