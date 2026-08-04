package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"gorm.io/gorm"
)

const (
	NotificationTypeJoinRequest      = "join_request"
	NotificationTypeChatConversation = "chat_conversation"
	NotificationDefaultPageSize      = 30
	NotificationMaxPageSize          = 50
)

type NotificationItem struct {
	Key        string
	Type       string
	ActivityAt int64

	RoomCode  string
	RoomTitle string

	RequestID        string
	ActorUserID      string
	ActorDisplayName string
	CreatedAt        int64
	ExpiresAt        int64

	ConversationID    string
	PeerUserID        string
	PeerDisplayName   string
	LatestMessageText string
	UnreadCount       int64
	LatestMessageAt   int64
}

type NotificationPage struct {
	Items      []NotificationItem
	TotalCount int64
	NextCursor string
}

type NotificationSvc struct {
	db  *gorm.DB
	now func() time.Time
}

type notificationCursor struct {
	Priority   int    `json:"priority"`
	ActivityAt int64  `json:"activityAt"`
	Key        string `json:"key"`
}

type joinRequestNotificationRow struct {
	RequestID        string
	ActorUserID      string
	ActorDisplayName string
	RoomCode         string
	RoomTitle        string
	CreatedAt        int64
	ExpiresAt        int64
}

type chatNotificationRow struct {
	ConversationID    string
	PeerUserID        string
	PeerDisplayName   string
	RoomCode          string
	RoomTitle         string
	LatestMessageText string
	UnreadCount       int64
	LatestMessageAt   int64
}

func NewNotificationSvc(db *gorm.DB) (*NotificationSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("notification database is required")
	}
	return &NotificationSvc{db: db, now: time.Now}, nil
}

func (s *NotificationSvc) List(userID, cursor string, limit int) (NotificationPage, error) {
	if s == nil || s.db == nil {
		return NotificationPage{}, fmt.Errorf("notification service is not initialized")
	}
	var user model.User
	if err := s.db.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotificationPage{}, errx.New(errc.ErrUserNotFound, nil)
		}
		return NotificationPage{}, err
	}
	limit, err := normalizeNotificationLimit(limit)
	if err != nil {
		return NotificationPage{}, err
	}
	position, err := decodeNotificationCursor(cursor)
	if err != nil {
		return NotificationPage{}, err
	}

	now := s.now().Unix()
	approvals, err := s.listJoinRequests(userID, now)
	if err != nil {
		return NotificationPage{}, err
	}
	chats, err := s.listUnreadChats(userID, now)
	if err != nil {
		return NotificationPage{}, err
	}
	items := make([]NotificationItem, 0, len(approvals)+len(chats))
	total := int64(len(approvals))
	for _, row := range approvals {
		items = append(items, NotificationItem{
			Key:              "join_request:" + row.RequestID,
			Type:             NotificationTypeJoinRequest,
			ActivityAt:       row.CreatedAt,
			RoomCode:         row.RoomCode,
			RoomTitle:        row.RoomTitle,
			RequestID:        row.RequestID,
			ActorUserID:      row.ActorUserID,
			ActorDisplayName: row.ActorDisplayName,
			CreatedAt:        row.CreatedAt,
			ExpiresAt:        row.ExpiresAt,
		})
	}
	for _, row := range chats {
		total += row.UnreadCount
		items = append(items, NotificationItem{
			Key:               "chat_conversation:" + row.ConversationID,
			Type:              NotificationTypeChatConversation,
			ActivityAt:        row.LatestMessageAt,
			RoomCode:          row.RoomCode,
			RoomTitle:         row.RoomTitle,
			ConversationID:    row.ConversationID,
			PeerUserID:        row.PeerUserID,
			PeerDisplayName:   row.PeerDisplayName,
			LatestMessageText: row.LatestMessageText,
			UnreadCount:       row.UnreadCount,
			LatestMessageAt:   row.LatestMessageAt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return notificationItemBefore(items[i], items[j]) })

	candidates := items
	if position != nil {
		candidates = make([]NotificationItem, 0, len(items))
		for _, item := range items {
			if notificationItemAfterCursor(item, *position) {
				candidates = append(candidates, item)
			}
		}
	}
	page := NotificationPage{Items: candidates, TotalCount: total}
	if len(candidates) > limit {
		page.Items = candidates[:limit]
		page.NextCursor, err = encodeNotificationCursor(page.Items[len(page.Items)-1])
		if err != nil {
			return NotificationPage{}, err
		}
	}
	return page, nil
}

func (s *NotificationSvc) listJoinRequests(userID string, now int64) ([]joinRequestNotificationRow, error) {
	var rows []joinRequestNotificationRow
	err := s.db.Table("join_requests AS request").
		Select("request.id AS request_id, request.user_id AS actor_user_id, request.display_name AS actor_display_name, room.code AS room_code, room.title AS room_title, request.created_at, request.expires_at").
		Joins("JOIN rooms AS room ON room.id = request.room_id").
		Joins("JOIN room_members AS owner_member ON owner_member.room_id = room.id AND owner_member.user_id = ? AND owner_member.role = ? AND owner_member.status = ?", userID, model.MemberRoleOwner, model.MemberStatusActive).
		Where("room.owner_user_id = ? AND room.join_mode = ? AND room.status = ? AND room.expires_at > ?", userID, model.JoinModeOwnerApproval, model.RoomStatusActive, now).
		Where("request.status = ? AND request.expires_at > ?", model.JoinRequestPending, now).
		Find(&rows).Error
	return rows, err
}

func (s *NotificationSvc) listUnreadChats(userID string, now int64) ([]chatNotificationRow, error) {
	const query = `
WITH visible_unread AS (
    SELECT
        message.room_id,
        message.conversation_id,
        message.sender_user_id,
        message.content_text,
        message.sequence,
        message.created_at,
        COUNT(*) OVER (PARTITION BY message.conversation_id) AS unread_count,
        ROW_NUMBER() OVER (PARTITION BY message.conversation_id ORDER BY message.sequence DESC) AS row_number
    FROM chat_messages AS message
    LEFT JOIN chat_read_states AS read_state
        ON read_state.conversation_id = message.conversation_id
       AND read_state.user_id = ?
    WHERE message.recipient_user_id = ?
      AND message.recalled_at IS NULL
      AND message.sequence > COALESCE(read_state.last_read_sequence, 0)
      AND NOT EXISTS (
          SELECT 1
          FROM chat_message_deletions AS deletion
          WHERE deletion.message_id = message.id
            AND deletion.user_id = ?
      )
)
SELECT
    unread.conversation_id,
    unread.sender_user_id AS peer_user_id,
    peer_member.display_name AS peer_display_name,
    room.code AS room_code,
    room.title AS room_title,
    unread.content_text AS latest_message_text,
    unread.unread_count,
    unread.created_at AS latest_message_at
FROM visible_unread AS unread
JOIN chat_conversations AS conversation
  ON conversation.id = unread.conversation_id
 AND conversation.room_id = unread.room_id
JOIN rooms AS room
  ON room.id = unread.room_id
 AND room.status = ?
 AND room.expires_at > ?
JOIN room_members AS current_member
  ON current_member.room_id = unread.room_id
 AND current_member.user_id = ?
 AND current_member.status = ?
JOIN room_members AS peer_member
  ON peer_member.room_id = unread.room_id
 AND peer_member.user_id = unread.sender_user_id
 AND peer_member.status = ?
WHERE unread.row_number = 1`

	var rows []chatNotificationRow
	err := s.db.Raw(query,
		userID,
		userID,
		userID,
		model.RoomStatusActive,
		now,
		userID,
		model.MemberStatusActive,
		model.MemberStatusActive,
	).Scan(&rows).Error
	return rows, err
}

func normalizeNotificationLimit(limit int) (int, error) {
	if limit == 0 {
		return NotificationDefaultPageSize, nil
	}
	if limit < 1 || limit > NotificationMaxPageSize {
		return 0, errx.New(errc.ErrParams, nil)
	}
	return limit, nil
}

func notificationPriority(notificationType string) int {
	if notificationType == NotificationTypeJoinRequest {
		return 0
	}
	return 1
}

func notificationItemBefore(left, right NotificationItem) bool {
	leftPriority := notificationPriority(left.Type)
	rightPriority := notificationPriority(right.Type)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if left.ActivityAt != right.ActivityAt {
		return left.ActivityAt > right.ActivityAt
	}
	return left.Key > right.Key
}

func notificationItemAfterCursor(item NotificationItem, cursor notificationCursor) bool {
	priority := notificationPriority(item.Type)
	if priority != cursor.Priority {
		return priority > cursor.Priority
	}
	if item.ActivityAt != cursor.ActivityAt {
		return item.ActivityAt < cursor.ActivityAt
	}
	return item.Key < cursor.Key
}

func encodeNotificationCursor(item NotificationItem) (string, error) {
	payload, err := json.Marshal(notificationCursor{Priority: notificationPriority(item.Type), ActivityAt: item.ActivityAt, Key: item.Key})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeNotificationCursor(value string) (*notificationCursor, error) {
	if value == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, errx.New(errc.ErrParams, nil)
	}
	var cursor notificationCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Priority < 0 || cursor.Priority > 1 || cursor.ActivityAt < 0 || cursor.Key == "" {
		return nil, errx.New(errc.ErrParams, nil)
	}
	return &cursor, nil
}
