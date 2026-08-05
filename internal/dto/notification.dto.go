package dto

type NotificationItemDTO struct {
	Key  string `json:"key"`
	Type string `json:"type"`

	RoomCode  string `json:"roomCode"`
	RoomTitle string `json:"roomTitle"`

	RequestID        string `json:"requestId,omitempty"`
	ActorUserID      string `json:"actorUserId,omitempty"`
	ActorDisplayName string `json:"actorDisplayName,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
	ExpiresAt        int64  `json:"expiresAt,omitempty"`

	ConversationID    string `json:"conversationId,omitempty"`
	PeerUserID        string `json:"peerUserId,omitempty"`
	PeerDisplayName   string `json:"peerDisplayName,omitempty"`
	LatestMessageText string `json:"latestMessageText,omitempty"`
	UnreadCount       int64  `json:"unreadCount,omitempty"`
	LatestMessageAt   int64  `json:"latestMessageAt,omitempty"`

	CounterpartUserID      string `json:"counterpartUserId,omitempty"`
	CounterpartDisplayName string `json:"counterpartDisplayName,omitempty"`
	LatestFileName         string `json:"latestFileName,omitempty"`
	FileCount              int64  `json:"fileCount,omitempty"`
	LatestFileEventAt      int64  `json:"latestFileEventAt,omitempty"`
	ReadToken              string `json:"readToken,omitempty"`
}

type NotificationPageDTO struct {
	Items      []NotificationItemDTO `json:"items"`
	TotalCount int64                 `json:"totalCount"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

type MarkNotificationReadRequest struct {
	Key       string `json:"key"`
	ReadToken string `json:"readToken"`
}

type MarkNotificationReadDTO struct {
	UpdatedCount int64 `json:"updatedCount"`
}
