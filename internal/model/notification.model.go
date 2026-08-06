package model

// NotificationRecord stores one file-notification occurrence for one user.
// Room, file, membership and recipient state remain the authority for whether
// the record is currently visible.
type NotificationRecord struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	UserID            string `gorm:"type:text;not null;index:idx_notification_records_user_unread;uniqueIndex:idx_notification_records_occurrence"`
	Type              string `gorm:"type:text;not null;index:idx_notification_records_user_unread;uniqueIndex:idx_notification_records_occurrence"`
	OccurrenceKey     string `gorm:"type:text;not null;uniqueIndex:idx_notification_records_occurrence"`
	RoomID            string `gorm:"type:text;not null;index:idx_notification_records_room_id"`
	FileID            string `gorm:"type:text;not null;index:idx_notification_records_file_id"`
	CounterpartUserID string `gorm:"type:text;not null;index:idx_notification_records_counterpart"`
	FileRecipientID   string `gorm:"type:text;index:idx_notification_records_file_recipient"`
	DeliveryVersion   int64  `gorm:"not null;default:0"`
	TrashCycleID      string `gorm:"type:text;index:idx_notification_records_trash_cycle"`
	TrashVersion      int64  `gorm:"not null;default:0"`
	RestoreRequestID  string `gorm:"type:text;index:idx_notification_records_restore_request"`
	SourceEventID     string `gorm:"type:text;not null;index:idx_notification_records_source_event"`
	OccurredAtMS      int64  `gorm:"not null;index:idx_notification_records_user_unread"`
	ReadAtMS          *int64 `gorm:"index:idx_notification_records_user_unread"`
}

func (NotificationRecord) TableName() string { return "notification_records" }
