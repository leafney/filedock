package dto

type SendChatMessageRequest struct {
	ClientMessageID string `json:"clientMessageId"`
	RecipientUserID string `json:"recipientUserId"`
	ContentText     string `json:"contentText"`
}

type MarkChatReadRequest struct {
	LastReadSequence int64 `json:"lastReadSequence"`
}

type ForwardChatMessageRequest struct {
	ClientMessageID  string   `json:"clientMessageId"`
	RecipientUserIDs []string `json:"recipientUserIds"`
}

type ChatMessageDTO struct {
	RoomCode          string `json:"roomCode"`
	ConversationID    string `json:"conversationId"`
	MessageID         string `json:"messageId"`
	ClientMessageID   string `json:"clientMessageId"`
	SenderUserID      string `json:"senderUserId"`
	RecipientUserID   string `json:"recipientUserId"`
	SenderDisplayName string `json:"senderDisplayName"`
	ContentText       string `json:"contentText"`
	IsForwarded       bool   `json:"isForwarded"`
	Sequence          int64  `json:"sequence"`
	CreatedAt         int64  `json:"createdAt"`
	RecalledAt        *int64 `json:"recalledAt,omitempty"`
	RecallDeadline    int64  `json:"recallDeadline,omitempty"`
	Read              bool   `json:"read"`
	CanCopy           bool   `json:"canCopy"`
	CanRecall         bool   `json:"canRecall"`
	CanRecallAndEdit  bool   `json:"canRecallAndEdit"`
	CanForward        bool   `json:"canForward"`
	CanDelete         bool   `json:"canDelete"`
}

type ChatConversationDTO struct {
	PeerUserID         string `json:"peerUserId"`
	PeerDisplayName    string `json:"peerDisplayName"`
	PeerRole           string `json:"peerRole"`
	PeerStatus         string `json:"peerStatus"`
	PeerOnlineStatus   string `json:"peerOnlineStatus"`
	UnreadCount        int    `json:"unreadCount"`
	LastMessageAt      *int64 `json:"lastMessageAt,omitempty"`
	LastMessagePreview string `json:"lastMessagePreview,omitempty"`
}

type ChatMessagePageDTO struct {
	Items               []ChatMessageDTO `json:"items"`
	PreviousCursor      *int64           `json:"previousCursor,omitempty"`
	HasMoreBefore       bool             `json:"hasMoreBefore"`
	CurrentReadSequence int64            `json:"currentReadSequence"`
	PeerReadSequence    int64            `json:"peerReadSequence"`
}

type ChatReadDTO struct {
	ConversationID   string `json:"conversationId"`
	PeerUserID       string `json:"peerUserId"`
	UserID           string `json:"userId"`
	LastReadSequence int64  `json:"lastReadSequence"`
	LastReadAt       int64  `json:"lastReadAt"`
}

type ChatSearchPageDTO struct {
	Items          []ChatMessageDTO `json:"items"`
	PreviousCursor *int64           `json:"previousCursor,omitempty"`
	HasMoreBefore  bool             `json:"hasMoreBefore"`
}
