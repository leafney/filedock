package model

// UploadBatch reserves room capacity for a group of files before bytes are sent.
type UploadBatch struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	RoomID            string `gorm:"type:text;not null;index:idx_upload_batches_room_id;uniqueIndex:idx_upload_batches_idempotency"`
	CreatorUserID     string `gorm:"type:text;not null;index:idx_upload_batches_creator_user_id;uniqueIndex:idx_upload_batches_idempotency"`
	IdempotencyKey    string `gorm:"type:text;not null;uniqueIndex:idx_upload_batches_idempotency"`
	Scope             string `gorm:"type:text;not null;index:idx_upload_batches_scope"`
	FileCount         int    `gorm:"not null"`
	DeclaredTotalSize int64  `gorm:"not null"`
	Status            string `gorm:"type:text;not null;index:idx_upload_batches_status"`
	CreatedAt         int64  `gorm:"not null;index:idx_upload_batches_created_at"`
	UpdatedAt         int64  `gorm:"not null"`
}

func (UploadBatch) TableName() string { return "upload_batches" }

// RoomFile stores metadata only. StorageName is always server generated.
type RoomFile struct {
	ID             string `gorm:"type:text;primaryKey;size:26"`
	RoomID         string `gorm:"type:text;not null;index:idx_room_files_room_status;index:idx_room_files_room_scope;index:idx_room_files_private_code"`
	BatchID        string `gorm:"type:text;not null;index:idx_room_files_batch_id"`
	UploaderUserID string `gorm:"type:text;not null;index:idx_room_files_uploader_user_id"`
	StorageName    string `gorm:"type:text;not null;uniqueIndex:idx_room_files_storage_name"`
	OriginalName   string `gorm:"type:text;not null"`
	DeclaredMIME   string `gorm:"type:text"`
	DetectedMIME   string `gorm:"type:text"`
	DeclaredSize   int64  `gorm:"not null"`
	ActualSize     int64  `gorm:"not null;default:0"`
	Scope          string `gorm:"type:text;not null;index:idx_room_files_room_scope"`
	PrivateCode    string `gorm:"type:text;index:idx_room_files_private_code"`
	Status         string `gorm:"type:text;not null;index:idx_room_files_room_status"`
	Progress       int    `gorm:"not null;default:0"`
	FailureCode    int    `gorm:"not null;default:0"`
	CreatedAt      int64  `gorm:"not null;index:idx_room_files_created_at"`
	StartedAt      *int64 `gorm:"index:idx_room_files_started_at"`
	CompletedAt    *int64 `gorm:"index:idx_room_files_completed_at"`
	FailedAt       *int64 `gorm:"index:idx_room_files_failed_at"`
}

func (RoomFile) TableName() string { return "room_files" }

type FileRecipient struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	FileID            string `gorm:"type:text;not null;uniqueIndex:idx_file_recipients_file_user"`
	RecipientUserID   string `gorm:"type:text;not null;uniqueIndex:idx_file_recipients_file_user;index:idx_file_recipients_user_status"`
	Status            string `gorm:"type:text;not null;index:idx_file_recipients_user_status"`
	SentAt            int64  `gorm:"not null"`
	AcceptedAt        *int64
	DeclinedAt        *int64
	FirstDownloadedAt *int64
	LastDownloadedAt  *int64
	DownloadCount     int `gorm:"not null;default:0"`
}

func (FileRecipient) TableName() string { return "file_recipients" }

// FileEvent is append-only application data. PayloadJSON must contain only
// internal safe facts; projection still happens before returning it to users.
type FileEvent struct {
	ID          string `gorm:"type:text;primaryKey;size:26"`
	RoomID      string `gorm:"type:text;not null;index:idx_file_events_room_cursor"`
	FileID      string `gorm:"type:text;index:idx_file_events_file_id"`
	BatchID     string `gorm:"type:text;index:idx_file_events_batch_id"`
	ActorUserID string `gorm:"type:text;index:idx_file_events_actor_user_id"`
	Type        string `gorm:"type:text;not null;index:idx_file_events_type"`
	PayloadJSON string `gorm:"type:text;not null;default:'{}'"`
	CreatedAt   int64  `gorm:"not null;index:idx_file_events_room_cursor"`
}

func (FileEvent) TableName() string { return "file_events" }

type DownloadTask struct {
	ID              string `gorm:"type:text;primaryKey;size:26"`
	RoomID          string `gorm:"type:text;not null;index:idx_download_tasks_room_status"`
	FileID          string `gorm:"type:text;not null;index:idx_download_tasks_file_id"`
	UserID          string `gorm:"type:text;not null;index:idx_download_tasks_user_status"`
	ExpectedSize    int64  `gorm:"not null"`
	TransferredSize int64  `gorm:"not null;default:0"`
	Status          string `gorm:"type:text;not null;index:idx_download_tasks_room_status;index:idx_download_tasks_user_status"`
	CreatedAt       int64  `gorm:"not null"`
	ExpiresAt       int64  `gorm:"not null;index:idx_download_tasks_expires_at"`
	StartedAt       *int64
	CompletedAt     *int64
	FailedAt        *int64
}

func (DownloadTask) TableName() string { return "download_tasks" }
