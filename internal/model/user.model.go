package model

type User struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	DisplayName       string `gorm:"type:text;not null"`
	DisplayNameKey    string `gorm:"type:text;not null;size:128;uniqueIndex:idx_users_display_name_key,where:display_name_key <> ''"`
	Status            string `gorm:"type:text;not null;index:idx_users_status"`
	DeviceHint        string `gorm:"type:text"`
	CreatedAt         int64  `gorm:"not null;index:idx_users_created_at"`
	UpdatedAt         int64  `gorm:"not null"`
	LastSeenAt        int64  `gorm:"not null;index:idx_users_last_seen_at"`
	IdentityExpiresAt int64  `gorm:"not null;index:idx_users_identity_expires_at"`
}

func (User) TableName() string { return "users" }
