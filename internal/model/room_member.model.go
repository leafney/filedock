package model

type RoomMember struct {
	ID             string `gorm:"type:text;primaryKey;size:26"`
	RoomID         string `gorm:"type:text;not null;uniqueIndex:idx_room_members_room_user"`
	UserID         string `gorm:"type:text;not null;uniqueIndex:idx_room_members_room_user"`
	Role           string `gorm:"type:text;not null;index:idx_room_members_role"`
	DisplayName    string `gorm:"type:text;not null"`
	Status         string `gorm:"type:text;not null;index:idx_room_members_status"`
	JoinedAt       int64  `gorm:"not null;index:idx_room_members_joined_at"`
	LastSeenAt     int64  `gorm:"not null;index:idx_room_members_last_seen_at"`
	LeftAt         *int64 `gorm:"index:idx_room_members_left_at"`
	KickedAt       *int64 `gorm:"index:idx_room_members_kicked_at"`
	KickedByUserID string `gorm:"type:text;size:26"`
}

func (RoomMember) TableName() string { return "room_members" }
