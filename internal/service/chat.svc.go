package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	ChatMessageMaxLength   = 2000
	ChatMessagePageSize    = 30
	ChatMessageContextSize = 31
	ChatMessageMaxPage     = 100
	ChatRecallWindow       = 2 * time.Minute
	ChatRateShortLimit     = 10
	ChatRateShortWindow    = 10 * time.Second
	ChatRateLongLimit      = 60
	ChatRateLongWindow     = time.Minute
)

type ChatSvc struct {
	db       *gorm.DB
	hub      *StreamHub
	limiter  *RateLimiter
	presence *PresenceSvc
	now      func() time.Time
}

type ChatMessageView struct {
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

type ChatConversationSummary struct {
	PeerUserID         string `json:"peerUserId"`
	PeerDisplayName    string `json:"peerDisplayName"`
	PeerRole           string `json:"peerRole"`
	PeerStatus         string `json:"peerStatus"`
	PeerOnlineStatus   string `json:"peerOnlineStatus"`
	UnreadCount        int    `json:"unreadCount"`
	LastMessageAt      *int64 `json:"lastMessageAt,omitempty"`
	LastMessagePreview string `json:"lastMessagePreview,omitempty"`
}

type ChatHistoryQuery struct {
	BeforeSequence *int64
	AfterSequence  *int64
	AroundSequence *int64
	Limit          int
}

type ChatMessagePage struct {
	Items               []ChatMessageView `json:"items"`
	PreviousCursor      *int64            `json:"previousCursor,omitempty"`
	HasMoreBefore       bool              `json:"hasMoreBefore"`
	NextCursor          *int64            `json:"nextCursor,omitempty"`
	HasMoreAfter        bool              `json:"hasMoreAfter"`
	CurrentReadSequence int64             `json:"currentReadSequence"`
	PeerReadSequence    int64             `json:"peerReadSequence"`
}

type ChatSearchQuery struct {
	Query          string
	BeforeSequence *int64
	Limit          int
}

type ChatSearchPage struct {
	Items          []ChatMessageView `json:"items"`
	PreviousCursor *int64            `json:"previousCursor,omitempty"`
	HasMoreBefore  bool              `json:"hasMoreBefore"`
}

type ChatReadView struct {
	ConversationID   string `json:"conversationId"`
	PeerUserID       string `json:"peerUserId"`
	UserID           string `json:"userId"`
	LastReadSequence int64  `json:"lastReadSequence"`
	LastReadAt       int64  `json:"lastReadAt"`
}

func NewChatSvc(db *gorm.DB, hub *StreamHub, limiter *RateLimiter, presence *PresenceSvc) (*ChatSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("chat database is required")
	}
	if hub == nil {
		return nil, fmt.Errorf("chat stream hub is required")
	}
	return &ChatSvc{db: db, hub: hub, limiter: limiter, presence: presence, now: time.Now}, nil
}

func (s *ChatSvc) Send(userID, roomCode, recipientID, clientMessageID, content string) (ChatMessageView, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return ChatMessageView{}, err
	}
	if err := s.validateRecipient(room.ID, userID, recipientID); err != nil {
		return ChatMessageView{}, err
	}
	clientMessageID, err = normalizeClientMessageID(clientMessageID)
	if err != nil {
		return ChatMessageView{}, err
	}
	content, err = normalizeChatContent(content)
	if err != nil {
		return ChatMessageView{}, err
	}
	var existing model.ChatMessage
	if err := s.db.Where("room_id = ? AND sender_user_id = ? AND recipient_user_id = ? AND client_message_id = ?", room.ID, userID, recipientID, clientMessageID).First(&existing).Error; err == nil {
		return s.messageView(room.Code, existing, userID), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ChatMessageView{}, err
	}
	if err := s.allowMessages(room.ID, userID, 1); err != nil {
		return ChatMessageView{}, err
	}

	now := s.now().Unix()
	var message model.ChatMessage
	created := false
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var txExisting model.ChatMessage
		if err := tx.Where("room_id = ? AND sender_user_id = ? AND recipient_user_id = ? AND client_message_id = ?", room.ID, userID, recipientID, clientMessageID).First(&txExisting).Error; err == nil {
			message = txExisting
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var sender model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&sender).Error; err != nil {
			return errxUserNotFound(err)
		}
		conversation, err := getOrCreateChatConversation(tx, room.ID, userID, recipientID, now)
		if err != nil {
			return err
		}
		message, err = createChatMessage(tx, conversation, room.ID, userID, recipientID, sender.DisplayName, clientMessageID, content, false, now)
		if err != nil {
			return err
		}
		created = true
		return nil
	}); err != nil {
		return ChatMessageView{}, err
	}
	view := s.messageView(room.Code, message, userID)
	if created {
		s.publishMessageCreated(view, message.SenderUserID, message.RecipientUserID)
	}
	return view, nil
}

func (s *ChatSvc) ListConversations(userID, roomCode string) ([]ChatConversationSummary, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return nil, err
	}
	var members []model.RoomMember
	if err := s.db.Where("room_id = ? AND status = ? AND user_id <> ?", room.ID, model.MemberStatusActive, userID).Order("joined_at ASC, id ASC").Find(&members).Error; err != nil {
		return nil, err
	}
	result := make([]ChatConversationSummary, 0, len(members))
	for _, member := range members {
		peerOnlineStatus := PresenceOffline
		if s.presence != nil {
			peerOnlineStatus = s.presence.Status(member.UserID)
		}
		summary := ChatConversationSummary{PeerUserID: member.UserID, PeerDisplayName: member.DisplayName, PeerRole: member.Role, PeerStatus: member.Status, PeerOnlineStatus: peerOnlineStatus}
		conversation, err := findChatConversation(s.db, room.ID, userID, member.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result = append(result, summary)
				continue
			}
			return nil, err
		}
		readSequence, err := getChatReadSequence(s.db, conversation.ID, userID)
		if err != nil {
			return nil, err
		}
		var unread int64
		unreadQuery := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.recipient_user_id = ? AND chat_messages.sequence > ? AND chat_messages.recalled_at IS NULL", conversation.ID, userID, readSequence)
		if err := unreadQuery.Count(&unread).Error; err != nil {
			return nil, err
		}
		summary.UnreadCount = int(unread)
		var latest model.ChatMessage
		if err := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ?", conversation.ID).Order("chat_messages.sequence DESC").First(&latest).Error; err == nil {
			lastAt := latest.CreatedAt
			summary.LastMessageAt = &lastAt
			if latest.RecalledAt == nil {
				summary.LastMessagePreview = latest.ContentText
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		result = append(result, summary)
	}
	return result, nil
}

func (s *ChatSvc) ListMessages(userID, roomCode, peerUserID string, query ChatHistoryQuery) (ChatMessagePage, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return ChatMessagePage{}, err
	}
	if err := s.validateRecipient(room.ID, userID, peerUserID); err != nil {
		return ChatMessagePage{}, err
	}
	if historyDirectionCount(query) > 1 {
		return ChatMessagePage{}, errcError(errc.ErrParams)
	}
	limit := normalizeChatLimit(query.Limit)
	conversation, err := findChatConversation(s.db, room.ID, userID, peerUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatMessagePage{Items: []ChatMessageView{}}, nil
		}
		return ChatMessagePage{}, err
	}
	var messages []model.ChatMessage
	hasMoreBefore := false
	hasMoreAfter := false
	if query.AroundSequence != nil {
		messages, hasMoreBefore, hasMoreAfter, err = s.loadAroundMessages(conversation.ID, userID, *query.AroundSequence)
	} else if query.AfterSequence != nil {
		messages, hasMoreBefore, hasMoreAfter, err = s.loadAfterMessages(conversation.ID, userID, *query.AfterSequence, limit)
	} else {
		messages, hasMoreBefore, hasMoreAfter, err = s.loadBeforeMessages(conversation.ID, userID, query.BeforeSequence, limit)
	}
	if err != nil {
		return ChatMessagePage{}, err
	}
	currentRead, err := getChatReadSequence(s.db, conversation.ID, userID)
	if err != nil {
		return ChatMessagePage{}, err
	}
	peerRead, err := getChatReadSequence(s.db, conversation.ID, peerUserID)
	if err != nil {
		return ChatMessagePage{}, err
	}
	views := make([]ChatMessageView, 0, len(messages))
	for _, message := range messages {
		views = append(views, s.messageView(room.Code, message, userID, peerRead))
	}
	page := ChatMessagePage{Items: views, HasMoreBefore: hasMoreBefore, HasMoreAfter: hasMoreAfter, CurrentReadSequence: currentRead, PeerReadSequence: peerRead}
	if hasMoreBefore && len(messages) > 0 {
		cursor := messages[0].Sequence
		page.PreviousCursor = &cursor
	}
	if hasMoreAfter && len(messages) > 0 {
		cursor := messages[len(messages)-1].Sequence
		page.NextCursor = &cursor
	}
	return page, nil
}

func (s *ChatSvc) MarkRead(userID, roomCode, peerUserID string, sequence int64) (ChatReadView, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return ChatReadView{}, err
	}
	if err := s.validateRecipient(room.ID, userID, peerUserID); err != nil {
		return ChatReadView{}, err
	}
	conversation, err := findChatConversation(s.db, room.ID, userID, peerUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatReadView{PeerUserID: peerUserID, UserID: userID}, nil
		}
		return ChatReadView{}, err
	}
	if sequence < 0 {
		sequence = 0
	}
	if sequence > conversation.LastSequence {
		sequence = conversation.LastSequence
	}
	now := s.now().Unix()
	var state model.ChatReadState
	advanced := false
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("conversation_id = ? AND user_id = ?", conversation.ID, userID).First(&state).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			state = model.ChatReadState{ConversationID: conversation.ID, UserID: userID, LastReadSequence: sequence, LastReadAt: now, UpdatedAt: now}
			advanced = sequence > 0
			return tx.Create(&state).Error
		}
		if err != nil {
			return err
		}
		if sequence <= state.LastReadSequence {
			return nil
		}
		advanced = true
		state.LastReadSequence = sequence
		state.LastReadAt = now
		state.UpdatedAt = now
		return tx.Save(&state).Error
	}); err != nil {
		return ChatReadView{}, err
	}
	if advanced {
		s.hub.PublishUsers([]string{userID, peerUserID}, "chat.read_updated", map[string]interface{}{"roomCode": room.Code, "conversationId": conversation.ID, "userId": userID, "peerUserId": peerUserID, "lastReadSequence": state.LastReadSequence, "lastReadAt": state.LastReadAt})
	}
	return ChatReadView{ConversationID: conversation.ID, PeerUserID: peerUserID, UserID: userID, LastReadSequence: state.LastReadSequence, LastReadAt: state.LastReadAt}, nil
}

func (s *ChatSvc) Recall(userID, roomCode, messageID string) (ChatMessageView, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return ChatMessageView{}, err
	}
	message, err := s.findVisibleMessage(userID, room.ID, messageID)
	if err != nil {
		return ChatMessageView{}, err
	}
	if message.SenderUserID != userID {
		return ChatMessageView{}, errcError(errc.ErrChatMessageState)
	}
	now := s.now()
	if message.RecalledAt != nil {
		return ChatMessageView{}, errcError(errc.ErrChatMessageState)
	}
	if now.Unix() > message.CreatedAt+int64(ChatRecallWindow/time.Second) {
		return ChatMessageView{}, errcError(errc.ErrChatRecallExpired)
	}
	recalledAt := now.Unix()
	result := s.db.Model(&model.ChatMessage{}).Where("id = ? AND room_id = ? AND sender_user_id = ? AND recalled_at IS NULL", message.ID, room.ID, userID).Updates(map[string]interface{}{"content_text": "", "recalled_at": recalledAt})
	if result.Error != nil {
		return ChatMessageView{}, result.Error
	}
	if result.RowsAffected != 1 {
		return ChatMessageView{}, errcError(errc.ErrChatMessageState)
	}
	message.ContentText = ""
	message.RecalledAt = &recalledAt
	view := s.messageView(room.Code, message, userID)
	s.hub.PublishUsers([]string{message.SenderUserID, message.RecipientUserID}, "chat.message_recalled", map[string]interface{}{"roomCode": room.Code, "conversationId": message.ConversationID, "messageId": message.ID, "sequence": message.Sequence, "recalledAt": recalledAt, "actorUserId": userID})
	return view, nil
}

func (s *ChatSvc) DeleteMessage(userID, roomCode, messageID string) error {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return err
	}
	// Deletion is idempotent. Look up the participant message without the
	// viewer's deletion filter so repeating the request remains successful.
	message, err := findParticipantMessage(s.db, userID, room.ID, messageID)
	if err != nil {
		return err
	}
	deletedAt := s.now().Unix()
	result := s.db.Where("message_id = ? AND user_id = ?", message.ID, userID).FirstOrCreate(&model.ChatMessageDeletion{MessageID: message.ID, UserID: userID, DeletedAt: deletedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		s.hub.PublishUser(userID, "chat.message_deleted_local", map[string]interface{}{"roomCode": room.Code, "conversationId": message.ConversationID, "messageId": message.ID, "sequence": message.Sequence})
	}
	return nil
}

func (s *ChatSvc) Forward(userID, roomCode, messageID, clientMessageID string, recipientIDs []string) ([]ChatMessageView, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return nil, err
	}
	clientMessageID, err = normalizeClientMessageID(clientMessageID)
	if err != nil {
		return nil, err
	}
	recipients := normalizeRecipientIDs(recipientIDs)
	if len(recipients) == 0 || len(recipients) > MaxRoomMembers-1 {
		return nil, errcError(errc.ErrChatRecipient)
	}
	for _, recipientID := range recipients {
		if err := s.validateRecipient(room.ID, userID, recipientID); err != nil {
			return nil, err
		}
	}
	// Validate the source before consuming any forwarding quota. This also
	// prevents an invalid or already hidden source from wasting a retry.
	source, err := s.findVisibleMessage(userID, room.ID, messageID)
	if err != nil {
		return nil, err
	}
	if source.RecalledAt != nil || source.ContentText == "" {
		return nil, errx.New(errc.ErrChatMessageState, nil)
	}

	// A retry may already have created some (or all) target messages. Only
	// missing targets consume quota; a fully idempotent retry is free.
	missing := 0
	for _, recipientID := range recipients {
		var existing model.ChatMessage
		queryErr := s.db.Where("room_id = ? AND sender_user_id = ? AND recipient_user_id = ? AND client_message_id = ?", room.ID, userID, recipientID, clientMessageID).First(&existing).Error
		if errors.Is(queryErr, gorm.ErrRecordNotFound) {
			missing++
			continue
		}
		if queryErr != nil {
			return nil, queryErr
		}
	}
	if missing > 0 {
		if err := s.allowMessages(room.ID, userID, missing); err != nil {
			return nil, err
		}
	}
	now := s.now().Unix()
	messages := make([]model.ChatMessage, 0, len(recipients))
	created := make([]model.ChatMessage, 0, len(recipients))
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		source, err := findVisibleMessageTx(tx, userID, room.ID, messageID)
		if err != nil {
			return err
		}
		if source.RecalledAt != nil || source.ContentText == "" {
			return errcError(errc.ErrChatMessageState)
		}
		var sender model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&sender).Error; err != nil {
			return errxUserNotFound(err)
		}
		for _, recipientID := range recipients {
			var existing model.ChatMessage
			if err := tx.Where("room_id = ? AND sender_user_id = ? AND recipient_user_id = ? AND client_message_id = ?", room.ID, userID, recipientID, clientMessageID).First(&existing).Error; err == nil {
				messages = append(messages, existing)
				continue
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			conversation, err := getOrCreateChatConversation(tx, room.ID, userID, recipientID, now)
			if err != nil {
				return err
			}
			message, err := createChatMessage(tx, conversation, room.ID, userID, recipientID, sender.DisplayName, clientMessageID, source.ContentText, true, now)
			if err != nil {
				return err
			}
			messages = append(messages, message)
			created = append(created, message)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	views := make([]ChatMessageView, 0, len(messages))
	for _, message := range messages {
		views = append(views, s.messageView(room.Code, message, userID))
	}
	for _, message := range created {
		s.publishMessageCreated(s.messageView(room.Code, message, userID), message.SenderUserID, message.RecipientUserID)
	}
	return views, nil
}

func (s *ChatSvc) Search(userID, roomCode, peerUserID string, query ChatSearchQuery) (ChatSearchPage, error) {
	room, _, err := s.activeMember(userID, roomCode)
	if err != nil {
		return ChatSearchPage{}, err
	}
	if err := s.validateRecipient(room.ID, userID, peerUserID); err != nil {
		return ChatSearchPage{}, err
	}
	term := strings.TrimSpace(query.Query)
	if term == "" || utf8.RuneCountInString(term) > 100 {
		return ChatSearchPage{}, errcError(errc.ErrChatContent)
	}
	limit := normalizeChatLimit(query.Limit)
	conversation, err := findChatConversation(s.db, room.ID, userID, peerUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatSearchPage{Items: []ChatMessageView{}}, nil
		}
		return ChatSearchPage{}, err
	}
	pattern := "%" + escapeLikePattern(term) + "%"
	db := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.recalled_at IS NULL AND chat_messages.content_text LIKE ? ESCAPE '\\'", conversation.ID, pattern)
	if query.BeforeSequence != nil {
		db = db.Where("chat_messages.sequence < ?", *query.BeforeSequence)
	}
	var messages []model.ChatMessage
	if err := db.Order("chat_messages.sequence DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return ChatSearchPage{}, err
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	peerRead, err := getChatReadSequence(s.db, conversation.ID, peerUserID)
	if err != nil {
		return ChatSearchPage{}, err
	}
	views := make([]ChatMessageView, 0, len(messages))
	for _, message := range messages {
		views = append(views, s.messageView(room.Code, message, userID, peerRead))
	}
	page := ChatSearchPage{Items: views, HasMoreBefore: hasMore}
	if hasMore && len(messages) > 0 {
		cursor := messages[len(messages)-1].Sequence
		page.PreviousCursor = &cursor
	}
	return page, nil
}

func (s *ChatSvc) activeMember(userID, roomCode string) (model.Room, model.RoomMember, error) {
	var room model.Room
	if err := s.db.Where("code = ?", strings.TrimSpace(roomCode)).First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Room{}, model.RoomMember{}, errcError(errc.ErrRoomNotFound)
		}
		return model.Room{}, model.RoomMember{}, err
	}
	if room.Status != model.RoomStatusActive {
		return model.Room{}, model.RoomMember{}, errcError(errc.ErrRoomNotActive)
	}
	var member model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Room{}, model.RoomMember{}, errcError(errc.ErrMemberRequired)
		}
		return model.Room{}, model.RoomMember{}, err
	}
	return room, member, nil
}

func (s *ChatSvc) validateRecipient(roomID, userID, recipientID string) error {
	if strings.TrimSpace(recipientID) == "" || recipientID == userID {
		return errcError(errc.ErrChatRecipient)
	}
	var member model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", roomID, recipientID, model.MemberStatusActive).First(&member).Error; err != nil {
		return errcError(errc.ErrChatRecipient)
	}
	return nil
}

func (s *ChatSvc) allowMessages(roomID, userID string, amount int) error {
	if s.limiter == nil {
		return nil
	}
	key := roomID + "\x00" + userID
	if allowed, _ := s.limiter.AllowNPair("chat", key, amount, ChatRateShortLimit, ChatRateShortWindow, ChatRateLongLimit, ChatRateLongWindow); !allowed {
		return errcError(errc.ErrChatRateLimited)
	}
	return nil
}

func (s *ChatSvc) findVisibleMessage(userID, roomID, messageID string) (model.ChatMessage, error) {
	return findVisibleMessageTx(s.db, userID, roomID, messageID)
}

func findParticipantMessage(db *gorm.DB, userID, roomID, messageID string) (model.ChatMessage, error) {
	var message model.ChatMessage
	if err := db.Where("chat_messages.id = ? AND chat_messages.room_id = ? AND (chat_messages.sender_user_id = ? OR chat_messages.recipient_user_id = ?)", messageID, roomID, userID, userID).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ChatMessage{}, errx.New(errc.ErrChatMessageNotFound, nil)
		}
		return model.ChatMessage{}, err
	}
	return message, nil
}

func findVisibleMessageTx(db *gorm.DB, userID, roomID, messageID string) (model.ChatMessage, error) {
	var message model.ChatMessage
	query := db.Where("chat_messages.id = ? AND chat_messages.room_id = ? AND (chat_messages.sender_user_id = ? OR chat_messages.recipient_user_id = ?)", messageID, roomID, userID, userID)
	query = chatVisibleMessageQuery(query, userID)
	if err := query.First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ChatMessage{}, errcError(errc.ErrChatMessageNotFound)
		}
		return model.ChatMessage{}, err
	}
	return message, nil
}

func getOrCreateChatConversation(tx *gorm.DB, roomID, userAID, userBID string, now int64) (model.ChatConversation, error) {
	userAID, userBID = canonicalChatPair(userAID, userBID)
	var conversation model.ChatConversation
	if err := tx.Where("room_id = ? AND user_a_id = ? AND user_b_id = ?", roomID, userAID, userBID).First(&conversation).Error; err == nil {
		return conversation, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ChatConversation{}, err
	}
	id, err := ulidx.New()
	if err != nil {
		return model.ChatConversation{}, err
	}
	conversation = model.ChatConversation{ID: id, RoomID: roomID, UserAID: userAID, UserBID: userBID, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&conversation).Error; err != nil {
		return model.ChatConversation{}, err
	}
	return conversation, nil
}

func findChatConversation(db *gorm.DB, roomID, userAID, userBID string) (model.ChatConversation, error) {
	userAID, userBID = canonicalChatPair(userAID, userBID)
	var conversation model.ChatConversation
	err := db.Where("room_id = ? AND user_a_id = ? AND user_b_id = ?", roomID, userAID, userBID).First(&conversation).Error
	return conversation, err
}

func createChatMessage(tx *gorm.DB, conversation model.ChatConversation, roomID, senderID, recipientID, senderDisplayName, clientMessageID, content string, forwarded bool, now int64) (model.ChatMessage, error) {
	sequence := conversation.LastSequence + 1
	result := tx.Model(&model.ChatConversation{}).Where("id = ? AND last_sequence = ?", conversation.ID, conversation.LastSequence).Updates(map[string]interface{}{"last_sequence": sequence, "updated_at": now})
	if result.Error != nil {
		return model.ChatMessage{}, result.Error
	}
	if result.RowsAffected != 1 {
		return model.ChatMessage{}, errcError(errc.ErrChatMessageState)
	}
	id, err := ulidx.New()
	if err != nil {
		return model.ChatMessage{}, err
	}
	message := model.ChatMessage{ID: id, RoomID: roomID, ConversationID: conversation.ID, SenderUserID: senderID, RecipientUserID: recipientID, SenderDisplayName: senderDisplayName, ClientMessageID: clientMessageID, ContentText: content, IsForwarded: forwarded, Sequence: sequence, CreatedAt: now}
	if err := tx.Create(&message).Error; err != nil {
		return model.ChatMessage{}, err
	}
	return message, nil
}

func (s *ChatSvc) loadBeforeMessages(conversationID, userID string, before *int64, limit int) ([]model.ChatMessage, bool, bool, error) {
	db := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ?", conversationID)
	if before != nil {
		db = db.Where("chat_messages.sequence < ?", *before)
	}
	var messages []model.ChatMessage
	if err := db.Order("chat_messages.sequence DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, false, false, err
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	reverseChatMessages(messages)
	if before != nil && len(messages) > 0 {
		hasMoreAfter, err := s.hasVisibleMessages(conversationID, userID, "chat_messages.sequence > ?", messages[len(messages)-1].Sequence)
		if err != nil {
			return nil, false, false, err
		}
		return messages, hasMore, hasMoreAfter, nil
	}
	return messages, hasMore, false, nil
}

func (s *ChatSvc) loadAfterMessages(conversationID, userID string, after int64, limit int) ([]model.ChatMessage, bool, bool, error) {
	db := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.sequence > ?", conversationID, after)
	var messages []model.ChatMessage
	if err := db.Order("chat_messages.sequence ASC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, false, false, err
	}
	hasMoreAfter := len(messages) > limit
	if hasMoreAfter {
		messages = messages[:limit]
	}
	hasMoreBefore := false
	if len(messages) > 0 {
		var err error
		hasMoreBefore, err = s.hasVisibleMessages(conversationID, userID, "chat_messages.sequence < ?", messages[0].Sequence)
		if err != nil {
			return nil, false, false, err
		}
	}
	return messages, hasMoreBefore, hasMoreAfter, nil
}

func (s *ChatSvc) loadAroundMessages(conversationID, userID string, around int64) ([]model.ChatMessage, bool, bool, error) {
	const sideLimit = (ChatMessageContextSize - 1) / 2
	var target model.ChatMessage
	targetQuery := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.sequence = ? AND chat_messages.recalled_at IS NULL", conversationID, around)
	if err := targetQuery.First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, false, errcError(errc.ErrChatMessageNotFound)
		}
		return nil, false, false, err
	}
	var before []model.ChatMessage
	beforeQuery := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.sequence < ?", conversationID, around)
	if err := beforeQuery.Order("chat_messages.sequence DESC").Limit(sideLimit + 1).Find(&before).Error; err != nil {
		return nil, false, false, err
	}
	hasMoreBefore := len(before) > sideLimit
	if hasMoreBefore {
		before = before[:sideLimit]
	}
	reverseChatMessages(before)
	var after []model.ChatMessage
	afterQuery := chatVisibleMessageQuery(s.db, userID).Where("chat_messages.conversation_id = ? AND chat_messages.sequence > ?", conversationID, around)
	if err := afterQuery.Order("chat_messages.sequence ASC").Limit(sideLimit + 1).Find(&after).Error; err != nil {
		return nil, false, false, err
	}
	hasMoreAfter := len(after) > sideLimit
	if hasMoreAfter {
		after = after[:sideLimit]
	}
	messages := append(before, target)
	messages = append(messages, after...)
	return messages, hasMoreBefore, hasMoreAfter, nil
}

func (s *ChatSvc) hasVisibleMessages(conversationID, userID, condition string, sequence int64) (bool, error) {
	var count int64
	db := chatVisibleMessageQuery(s.db.Model(&model.ChatMessage{}), userID).Where("chat_messages.conversation_id = ?", conversationID).Where(condition, sequence)
	if err := db.Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ChatSvc) messageView(roomCode string, message model.ChatMessage, viewerID string, peerRead ...int64) ChatMessageView {
	now := s.now().Unix()
	view := ChatMessageView{RoomCode: roomCode, ConversationID: message.ConversationID, MessageID: message.ID, ClientMessageID: message.ClientMessageID, SenderUserID: message.SenderUserID, RecipientUserID: message.RecipientUserID, SenderDisplayName: message.SenderDisplayName, ContentText: message.ContentText, IsForwarded: message.IsForwarded, Sequence: message.Sequence, CreatedAt: message.CreatedAt, RecalledAt: message.RecalledAt, CanCopy: message.RecalledAt == nil && message.ContentText != "", CanForward: message.RecalledAt == nil && message.ContentText != "", CanDelete: viewerID == message.SenderUserID || viewerID == message.RecipientUserID}
	if message.RecalledAt != nil {
		view.ContentText = ""
	}
	if message.SenderUserID == viewerID && message.RecalledAt == nil {
		view.RecallDeadline = message.CreatedAt + int64(ChatRecallWindow/time.Second)
		view.CanRecall = now <= view.RecallDeadline
		view.CanRecallAndEdit = view.CanRecall
		if len(peerRead) > 0 {
			view.Read = peerRead[0] >= message.Sequence
		}
	}
	return view
}

func (s *ChatSvc) publishMessageCreated(view ChatMessageView, userIDs ...string) {
	payload := map[string]interface{}{"roomCode": view.RoomCode, "conversationId": view.ConversationID, "messageId": view.MessageID, "clientMessageId": view.ClientMessageID, "senderUserId": view.SenderUserID, "recipientUserId": view.RecipientUserID, "senderDisplayName": view.SenderDisplayName, "contentText": view.ContentText, "isForwarded": view.IsForwarded, "sequence": view.Sequence, "createdAt": view.CreatedAt}
	s.hub.PublishUsers(uniqueChatUsers(userIDs), "chat.message_created", payload)
}

func chatVisibleMessageQuery(db *gorm.DB, userID string) *gorm.DB {
	return db.Where("NOT EXISTS (SELECT 1 FROM chat_message_deletions AS deletions WHERE deletions.message_id = chat_messages.id AND deletions.user_id = ?)", userID)
}

func getChatReadSequence(db *gorm.DB, conversationID, userID string) (int64, error) {
	var state model.ChatReadState
	if err := db.Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return state.LastReadSequence, nil
}

func canonicalChatPair(userAID, userBID string) (string, string) {
	if userAID <= userBID {
		return userAID, userBID
	}
	return userBID, userAID
}

func normalizeChatContent(content string) (string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > ChatMessageMaxLength {
		return "", errcError(errc.ErrChatContent)
	}
	return content, nil
}

func normalizeClientMessageID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 100 {
		return "", errcError(errc.ErrParams)
	}
	return value, nil
}

func normalizeRecipientIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeChatLimit(limit int) int {
	if limit <= 0 {
		return ChatMessagePageSize
	}
	if limit > ChatMessageMaxPage {
		return ChatMessageMaxPage
	}
	return limit
}

func historyDirectionCount(query ChatHistoryQuery) int {
	count := 0
	if query.BeforeSequence != nil {
		count++
	}
	if query.AfterSequence != nil {
		count++
	}
	if query.AroundSequence != nil {
		count++
	}
	return count
}

func reverseChatMessages(messages []model.ChatMessage) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func uniqueChatUsers(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func errcError(code int) error { return errx.New(code, nil) }

func errxUserNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcError(errc.ErrUserNotFound)
	}
	return err
}
