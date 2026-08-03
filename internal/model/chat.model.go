package model

// ChatConversation identifies one private conversation inside one room.
// UserAID and UserBID are stored in stable lexical order.
type ChatConversation struct {
	ID           string `gorm:"type:text;primaryKey;size:26"`
	RoomID       string `gorm:"type:text;not null;index:idx_chat_conversations_room_id;uniqueIndex:idx_chat_conversations_room_users"`
	UserAID      string `gorm:"type:text;not null;uniqueIndex:idx_chat_conversations_room_users"`
	UserBID      string `gorm:"type:text;not null;uniqueIndex:idx_chat_conversations_room_users"`
	LastSequence int64  `gorm:"not null;default:0"`
	CreatedAt    int64  `gorm:"not null;index:idx_chat_conversations_created_at"`
	UpdatedAt    int64  `gorm:"not null;index:idx_chat_conversations_updated_at"`
}

func (ChatConversation) TableName() string { return "chat_conversations" }

// ChatMessage stores one text message for one recipient. Forwarding to
// multiple members creates one independent row per recipient.
type ChatMessage struct {
	ID                string `gorm:"type:text;primaryKey;size:26"`
	RoomID            string `gorm:"type:text;not null;index:idx_chat_messages_room_id;uniqueIndex:idx_chat_messages_idempotency"`
	ConversationID    string `gorm:"type:text;not null;index:idx_chat_messages_conversation_seq;uniqueIndex:idx_chat_messages_conversation_seq"`
	SenderUserID      string `gorm:"type:text;not null;index:idx_chat_messages_sender_id;uniqueIndex:idx_chat_messages_idempotency"`
	RecipientUserID   string `gorm:"type:text;not null;index:idx_chat_messages_recipient_id;uniqueIndex:idx_chat_messages_idempotency"`
	SenderDisplayName string `gorm:"type:text;not null"`
	ClientMessageID   string `gorm:"type:text;not null;uniqueIndex:idx_chat_messages_idempotency"`
	ContentText       string `gorm:"type:text;not null"`
	IsForwarded       bool   `gorm:"not null;default:false"`
	Sequence          int64  `gorm:"not null;uniqueIndex:idx_chat_messages_conversation_seq"`
	CreatedAt         int64  `gorm:"not null;index:idx_chat_messages_created_at"`
	RecalledAt        *int64 `gorm:"index:idx_chat_messages_recalled_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// ChatReadState stores the last sequence a user has read in a conversation.
type ChatReadState struct {
	ConversationID   string `gorm:"type:text;primaryKey;size:26"`
	UserID           string `gorm:"type:text;primaryKey;size:26"`
	LastReadSequence int64  `gorm:"not null;default:0"`
	LastReadAt       int64  `gorm:"not null;default:0"`
	UpdatedAt        int64  `gorm:"not null"`
}

func (ChatReadState) TableName() string { return "chat_read_states" }

// ChatMessageDeletion hides a message for one user without changing the
// message projection seen by the other participant.
type ChatMessageDeletion struct {
	MessageID string `gorm:"type:text;primaryKey;size:26"`
	UserID    string `gorm:"type:text;primaryKey;size:26"`
	DeletedAt int64  `gorm:"not null"`
}

func (ChatMessageDeletion) TableName() string { return "chat_message_deletions" }
