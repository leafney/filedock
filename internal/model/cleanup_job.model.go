package model

type CleanupJob struct {
	ID                   string `gorm:"type:text;primaryKey;size:26"`
	RoomID               string `gorm:"type:text;not null;uniqueIndex:idx_cleanup_jobs_room_id"`
	Status               string `gorm:"type:text;not null;index:idx_cleanup_jobs_status"`
	Phase                string `gorm:"type:text;not null"`
	AttemptCount         int    `gorm:"not null"`
	ScheduledAt          int64  `gorm:"not null;index:idx_cleanup_jobs_scheduled_at"`
	StartedAt            *int64 `gorm:"index:idx_cleanup_jobs_started_at"`
	FinishedAt           *int64 `gorm:"index:idx_cleanup_jobs_finished_at"`
	LastError            string `gorm:"type:text"`
	ErrorSummary         string `gorm:"type:text"`
	TotalMembers         int    `gorm:"not null"`
	CleanedMembers       int    `gorm:"not null"`
	TotalJoinRequests    int    `gorm:"not null"`
	CleanedJoinRequests  int    `gorm:"not null"`
	TotalNotifications   int    `gorm:"not null"`
	CleanedNotifications int    `gorm:"not null"`
}

func (CleanupJob) TableName() string { return "cleanup_jobs" }
