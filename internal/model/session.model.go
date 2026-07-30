package model

type Session struct {
	ID             string `gorm:"type:text;primaryKey;size:26"`
	UserID         string `gorm:"type:text;not null;index:idx_sessions_user_id"`
	JWTID          string `gorm:"type:text;not null;index:idx_sessions_jwt_id"`
	CreatedAt      int64  `gorm:"not null"`
	UpdatedAt      int64  `gorm:"not null"`
	LastActivityAt int64  `gorm:"not null;index:idx_sessions_last_activity_at"`
	ExpiresAt      int64  `gorm:"not null;index:idx_sessions_expires_at"`
	RevokedAt      *int64 `gorm:"index:idx_sessions_revoked_at"`
}

func (Session) TableName() string { return "sessions" }
