package model

const (
	UserStatusActive  = "active"
	UserStatusRevoked = "revoked"
	UserStatusExpired = "expired"

	SessionStatusActive = "active"

	RoomStatusActive     = "active"
	RoomStatusDestroying = "destroying"
	RoomStatusDestroyed  = "destroyed"

	JoinModeOpen          = "open"
	JoinModePassword      = "password"
	JoinModeOwnerApproval = "owner_approval"

	MemberRoleOwner  = "owner"
	MemberRoleMember = "member"

	MemberStatusActive    = "active"
	MemberStatusLeft      = "left"
	MemberStatusKicked    = "kicked"
	MemberStatusDestroyed = "destroyed"

	JoinRequestPending   = "pending"
	JoinRequestApproved  = "approved"
	JoinRequestRejected  = "rejected"
	JoinRequestCancelled = "cancelled"
	JoinRequestExpired   = "expired"

	CleanupPending      = "cleanup_pending"
	CleanupRunning      = "cleanup_running"
	CleanupRetryWaiting = "cleanup_retry_waiting"
	CleanupSucceeded    = "cleanup_succeeded"
	CleanupFailed       = "cleanup_failed"

	FileScopeShared = "shared"
	FileScopeDirect = "direct"

	UploadBatchPending   = "pending"
	UploadBatchUploading = "uploading"
	UploadBatchCompleted = "completed"
	UploadBatchPartial   = "partial"
	UploadBatchFailed    = "failed"

	FileStatusReserved  = "reserved"
	FileStatusUploading = "uploading"
	FileStatusAvailable = "available"
	FileStatusFailed    = "failed"
	FileStatusCancelled = "cancelled"

	RecipientPending    = "pending"
	RecipientAccepted   = "accepted"
	RecipientDeclined   = "declined"
	RecipientDownloaded = "downloaded"

	DownloadTaskPending   = "pending"
	DownloadTaskStreaming = "streaming"
	DownloadTaskCompleted = "completed"
	DownloadTaskFailed    = "failed"
	DownloadTaskExpired   = "expired"
)
