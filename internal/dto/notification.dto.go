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
}

type NotificationPageDTO struct {
	Items      []NotificationItemDTO `json:"items"`
	TotalCount int64                 `json:"totalCount"`
	NextCursor string                `json:"nextCursor,omitempty"`
}
