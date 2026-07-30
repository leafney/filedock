package model

type Room struct {
	ID             string `gorm:"type:text;primaryKey;size:26"`
	Code           string `gorm:"type:text;not null;size:4;index:idx_rooms_code"`
	Title          string `gorm:"type:text;not null"`
	OwnerUserID    string `gorm:"type:text;not null;index:idx_rooms_owner_user_id"`
	JoinMode       string `gorm:"type:text;not null;index:idx_rooms_join_mode"`
	PINHash        string `gorm:"type:text"`
	Status         string `gorm:"type:text;not null;index:idx_rooms_status"`
	CreatedAt      int64  `gorm:"not null;index:idx_rooms_created_at"`
	ExpiresAt      int64  `gorm:"not null;index:idx_rooms_expires_at"`
	ExtendedAt     *int64 `gorm:"index:idx_rooms_extended_at"`
	DestroyingAt   *int64 `gorm:"index:idx_rooms_destroying_at"`
	DestroyAt      *int64 `gorm:"index:idx_rooms_destroy_at"`
	DestroyedAt    *int64 `gorm:"index:idx_rooms_destroyed_at"`
	CodeReusableAt *int64 `gorm:"index:idx_rooms_code_reusable_at"`
}

func (Room) TableName() string { return "rooms" }
