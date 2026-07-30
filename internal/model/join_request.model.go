package model

type JoinRequest struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	RoomID            string `gorm:"type:text;not null;index:idx_join_requests_room_status,priority:1"`
	UserID            string `gorm:"type:text;not null;index:idx_join_requests_user_status,priority:1"`
	DisplayName       string `gorm:"type:text;not null"`
	Status            string `gorm:"type:text;not null;index:idx_join_requests_room_status,priority:2;index:idx_join_requests_user_status,priority:2"`
	CreatedAt         int64  `gorm:"not null;index:idx_join_requests_created_at"`
	ExpiresAt         int64  `gorm:"not null;index:idx_join_requests_expires_at"`
	ProcessedAt       *int64 `gorm:"index:idx_join_requests_processed_at"`
	ProcessedByUserID string `gorm:"type:text;size:26"`
	CancelledAt       *int64 `gorm:"index:idx_join_requests_cancelled_at"`
	NextAllowedAt     *int64 `gorm:"index:idx_join_requests_next_allowed_at"`
}

func (JoinRequest) TableName() string { return "join_requests" }
