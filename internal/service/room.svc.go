package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/passwordx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	RoomTTL            = 12 * time.Hour
	RoomExtendTTL      = 12 * time.Hour
	RoomDestroyDelay   = 10 * time.Second
	RoomCodeCooldown   = 10 * time.Minute
	MaxOwnedRooms      = 1
	MaxJoinedRooms     = 5
	MaxRoomMembers     = 50
	ApprovalWait       = 10 * time.Minute
	ApprovalRejectWait = 10 * time.Minute
	ApprovalCancelWait = time.Minute
	MaxUserPending     = 5
	MaxRoomPending     = 50
	RoomCodeMin        = 1
	RoomCodeMax        = 9999
)

type RoomSummary struct {
	RoomID    string
	RoomCode  string
	Title     string
	Status    string
	Role      string
	ExpiresAt int64
}

type RoomMemberView struct {
	UserID       string
	DisplayName  string
	Role         string
	Status       string
	JoinedAt     int64
	OnlineStatus string
}

type RoomSnapshot struct {
	RoomID              string
	RoomCode            string
	Title               string
	Status              string
	JoinMode            string
	Role                string
	ExpiresAt           int64
	CanExtend           bool
	DestroyAt           *int64
	Members             []RoomMemberView
	PendingRequestCount int
}

type RoomJoinInfo struct {
	RoomCode       string
	Title          string
	JoinMode       string
	AlreadyMember  bool
	PendingRequest bool
}

type JoinRequestView struct {
	RequestID   string
	RoomCode    string
	UserID      string
	DisplayName string
	Status      string
	CreatedAt   int64
	ExpiresAt   int64
}

type RoomSvc struct {
	db       *gorm.DB
	now      func() time.Time
	hub      *StreamHub
	presence *PresenceSvc
}

func NewRoomSvc(db *gorm.DB, hub *StreamHub, presences ...*PresenceSvc) (*RoomSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("room database is required")
	}
	var presence *PresenceSvc
	if len(presences) > 0 {
		presence = presences[0]
	}
	return &RoomSvc{db: db, now: time.Now, hub: hub, presence: presence}, nil
}

func (s *RoomSvc) Create(userID, joinMode, pin, pinConfirmation string) (RoomSnapshot, error) {
	if s == nil || s.db == nil {
		return RoomSnapshot{}, fmt.Errorf("room service is nil")
	}
	now := s.now()
	if joinMode == "" {
		joinMode = model.JoinModeOpen
	}
	if joinMode != model.JoinModeOpen && joinMode != model.JoinModePassword && joinMode != model.JoinModeOwnerApproval {
		return RoomSnapshot{}, errx.New(errc.ErrJoinModeMismatch, nil)
	}
	var pinHash string
	if joinMode == model.JoinModePassword {
		if err := validatePIN(pin); err != nil {
			return RoomSnapshot{}, err
		}
		if pin != pinConfirmation {
			return RoomSnapshot{}, errx.New(errc.ErrPINConfirmation, nil)
		}
		var err error
		pinHash, err = passwordx.Hash(pin)
		if err != nil {
			return RoomSnapshot{}, fmt.Errorf("hash room pin: %w", err)
		}
	} else if pin != "" || pinConfirmation != "" {
		return RoomSnapshot{}, errx.New(errc.ErrJoinModeMismatch, nil)
	}
	roomID, err := ulidx.New()
	if err != nil {
		return RoomSnapshot{}, fmt.Errorf("generate room id: %w", err)
	}
	memberID, err := ulidx.New()
	if err != nil {
		return RoomSnapshot{}, fmt.Errorf("generate owner member id: %w", err)
	}
	var roomCode string
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(errc.ErrUserNotFound, nil)
			}
			return err
		}
		var owned int64
		if err := tx.Model(&model.Room{}).Where("owner_user_id = ? AND status IN ?", userID, []string{model.RoomStatusActive, model.RoomStatusDestroying}).Count(&owned).Error; err != nil {
			return err
		}
		if owned >= MaxOwnedRooms {
			return errx.New(errc.ErrOwnedRoomLimit, nil)
		}
		code, err := allocateRoomCode(tx, now.Unix())
		if err != nil {
			return err
		}
		roomCode = code
		expiresAt := now.Add(RoomTTL).Unix()
		room := model.Room{ID: roomID, Code: roomCode, Title: "房间 " + roomCode, OwnerUserID: userID, JoinMode: joinMode, PINHash: pinHash, Status: model.RoomStatusActive, CreatedAt: now.Unix(), ExpiresAt: expiresAt}
		member := model.RoomMember{ID: memberID, RoomID: roomID, UserID: userID, Role: model.MemberRoleOwner, DisplayName: user.DisplayName, Status: model.MemberStatusActive, JoinedAt: now.Unix(), LastSeenAt: now.Unix()}
		if err := tx.Create(&room).Error; err != nil {
			return fmt.Errorf("create room: %w", err)
		}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}
		return nil
	}); err != nil {
		return RoomSnapshot{}, err
	}
	snapshot, err := s.Snapshot(userID, roomCode)
	if err == nil {
		s.publishRoom(roomID, "room.member_joined", map[string]interface{}{"roomCode": roomCode, "userId": userID, "role": model.MemberRoleOwner})
	}
	return snapshot, err
}

func allocateRoomCode(tx *gorm.DB, now int64) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		value, err := rand.Int(rand.Reader, big.NewInt(RoomCodeMax))
		if err != nil {
			return "", fmt.Errorf("randomize room code: %w", err)
		}
		code := fmt.Sprintf("%04d", value.Int64()+RoomCodeMin)
		available, err := roomCodeAvailable(tx, code, now)
		if err != nil {
			return "", err
		}
		if available {
			return code, nil
		}
	}
	start, err := rand.Int(rand.Reader, big.NewInt(RoomCodeMax))
	if err != nil {
		return "", fmt.Errorf("randomize room scan start: %w", err)
	}
	for offset := int64(0); offset < RoomCodeMax; offset++ {
		value := (start.Int64() + offset) % RoomCodeMax
		code := fmt.Sprintf("%04d", value+RoomCodeMin)
		available, err := roomCodeAvailable(tx, code, now)
		if err != nil {
			return "", err
		}
		if available {
			return code, nil
		}
	}
	return "", errx.New(errc.ErrRoomCodeExhausted, nil)
}

func roomCodeAvailable(tx *gorm.DB, code string, now int64) (bool, error) {
	var count int64
	if err := tx.Model(&model.Room{}).Where("code = ? AND (status IN ? OR (status = ? AND (code_reusable_at IS NULL OR code_reusable_at > ? OR NOT EXISTS (SELECT 1 FROM cleanup_jobs WHERE cleanup_jobs.room_id = rooms.id AND cleanup_jobs.status = ?))) )", code, []string{model.RoomStatusActive, model.RoomStatusDestroying}, model.RoomStatusDestroyed, now, model.CleanupSucceeded).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check room code: %w", err)
	}
	return count == 0, nil
}

func (s *RoomSvc) List(userID string) ([]RoomSummary, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("room service is nil")
	}
	var memberships []model.RoomMember
	if err := s.db.Where("user_id = ? AND status = ?", userID, model.MemberStatusActive).Find(&memberships).Error; err != nil {
		return nil, err
	}
	result := make([]RoomSummary, 0, len(memberships))
	for _, membership := range memberships {
		var room model.Room
		if err := s.db.Where("id = ? AND status IN ?", membership.RoomID, []string{model.RoomStatusActive, model.RoomStatusDestroying}).First(&room).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		result = append(result, RoomSummary{RoomID: room.ID, RoomCode: room.Code, Title: room.Title, Status: room.Status, Role: membership.Role, ExpiresAt: room.ExpiresAt})
	}
	return result, nil
}

func (s *RoomSvc) JoinInfo(userID, code string) (RoomJoinInfo, error) {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return RoomJoinInfo{}, err
	}
	info := RoomJoinInfo{RoomCode: room.Code, Title: room.Title, JoinMode: room.JoinMode}
	if userID == "" {
		return info, nil
	}
	var member model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&member).Error; err == nil {
		info.AlreadyMember = true
	}
	var request model.JoinRequest
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.JoinRequestPending).First(&request).Error; err == nil {
		info.PendingRequest = true
	}
	return info, nil
}

func (s *RoomSvc) Join(userID, code string, confirmed bool, pin string) (RoomSnapshot, error) {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return RoomSnapshot{}, err
	}
	var existing model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&existing).Error; err == nil {
		return s.Snapshot(userID, code)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return RoomSnapshot{}, err
	}
	if room.JoinMode == model.JoinModeOwnerApproval {
		return RoomSnapshot{}, errx.New(errc.ErrJoinModeMismatch, nil)
	}
	if room.JoinMode == model.JoinModeOpen && !confirmed {
		return RoomSnapshot{}, errx.New(errc.ErrConflict, nil)
	}
	if room.JoinMode == model.JoinModePassword {
		if err := validatePIN(pin); err != nil {
			return RoomSnapshot{}, err
		}
		if err := passwordx.Compare(room.PINHash, pin); err != nil {
			return RoomSnapshot{}, errx.New(errc.ErrRoomPIN, nil)
		}
	}
	now := s.now().Unix()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(errc.ErrUserNotFound, nil)
			}
			return err
		}
		var member model.RoomMember
		memberErr := tx.Where("room_id = ? AND user_id = ?", room.ID, userID).First(&member).Error
		if memberErr == nil {
			if member.Status == model.MemberStatusActive {
				return nil
			}
			if member.Status == model.MemberStatusKicked {
				return errx.New(errc.ErrKicked, nil)
			}
		} else if !errors.Is(memberErr, gorm.ErrRecordNotFound) {
			return memberErr
		}
		var joined int64
		if err := tx.Model(&model.RoomMember{}).Where("user_id = ? AND role = ? AND status = ?", userID, model.MemberRoleMember, model.MemberStatusActive).Count(&joined).Error; err != nil {
			return err
		}
		if joined >= MaxJoinedRooms {
			return errx.New(errc.ErrJoinedRoomLimit, nil)
		}
		var members int64
		if err := tx.Model(&model.RoomMember{}).Where("room_id = ? AND status = ?", room.ID, model.MemberStatusActive).Count(&members).Error; err != nil {
			return err
		}
		if members >= MaxRoomMembers {
			return errx.New(errc.ErrRoomFull, nil)
		}
		if memberErr == nil {
			if err := tx.Model(&model.RoomMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{"status": model.MemberStatusActive, "display_name": user.DisplayName, "joined_at": now, "last_seen_at": now, "left_at": nil, "kicked_at": nil, "kicked_by_user_id": ""}).Error; err != nil {
				return err
			}
			return nil
		}
		memberID, err := ulidx.New()
		if err != nil {
			return err
		}
		return tx.Create(&model.RoomMember{ID: memberID, RoomID: room.ID, UserID: userID, Role: model.MemberRoleMember, DisplayName: user.DisplayName, Status: model.MemberStatusActive, JoinedAt: now, LastSeenAt: now}).Error
	}); err != nil {
		return RoomSnapshot{}, err
	}
	snapshot, err := s.Snapshot(userID, code)
	if err == nil {
		s.publishRoom(room.ID, "room.member_joined", map[string]interface{}{"roomCode": code, "userId": userID, "role": model.MemberRoleMember})
	}
	return snapshot, err
}

func (s *RoomSvc) Snapshot(userID, code string) (RoomSnapshot, error) {
	room, err := s.findRoom(code)
	if err != nil {
		return RoomSnapshot{}, err
	}
	if room.Status == model.RoomStatusDestroyed {
		return RoomSnapshot{}, errx.New(errc.ErrRoomNotFound, nil)
	}
	var current model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RoomSnapshot{}, errx.New(errc.ErrMemberRequired, nil)
		}
		return RoomSnapshot{}, err
	}
	var members []model.RoomMember
	if err := s.db.Where("room_id = ? AND status = ?", room.ID, model.MemberStatusActive).Order("joined_at ASC").Find(&members).Error; err != nil {
		return RoomSnapshot{}, err
	}
	views := make([]RoomMemberView, 0, len(members))
	for _, member := range members {
		onlineStatus := PresenceOffline
		if s.presence != nil {
			onlineStatus = s.presence.Status(member.UserID)
		}
		views = append(views, RoomMemberView{UserID: member.UserID, DisplayName: member.DisplayName, Role: member.Role, Status: member.Status, JoinedAt: member.JoinedAt, OnlineStatus: onlineStatus})
	}
	var pending int64
	if current.Role == model.MemberRoleOwner {
		if err := s.db.Model(&model.JoinRequest{}).Where("room_id = ? AND status = ?", room.ID, model.JoinRequestPending).Count(&pending).Error; err != nil {
			return RoomSnapshot{}, err
		}
	}
	return RoomSnapshot{RoomID: room.ID, RoomCode: room.Code, Title: room.Title, Status: room.Status, JoinMode: room.JoinMode, Role: current.Role, ExpiresAt: room.ExpiresAt, CanExtend: current.Role == model.MemberRoleOwner && room.Status == model.RoomStatusActive && room.ExtendedAt == nil, DestroyAt: room.DestroyAt, Members: views, PendingRequestCount: int(pending)}, nil
}

func (s *RoomSvc) Members(userID, code string) ([]RoomMemberView, error) {
	snapshot, err := s.Snapshot(userID, code)
	if err != nil {
		return nil, err
	}
	return snapshot.Members, nil
}

func (s *RoomSvc) Extend(userID, code string) (int64, error) {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return 0, err
	}
	now := s.now().Unix()
	result := s.db.Model(&model.Room{}).Where("id = ? AND owner_user_id = ? AND status = ? AND extended_at IS NULL", room.ID, userID, model.RoomStatusActive).Updates(map[string]interface{}{"expires_at": room.ExpiresAt + int64(RoomExtendTTL/time.Second), "extended_at": now})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		var owner model.RoomMember
		if err := s.db.Where("room_id = ? AND user_id = ? AND role = ? AND status = ?", room.ID, userID, model.MemberRoleOwner, model.MemberStatusActive).First(&owner).Error; err != nil {
			return 0, errx.New(errc.ErrOwnerRequired, nil)
		}
		return 0, errx.New(errc.ErrRoomAlreadyExtended, nil)
	}
	expiresAt := room.ExpiresAt + int64(RoomExtendTTL/time.Second)
	s.publishRoom(room.ID, "room.extended", map[string]interface{}{"roomCode": room.Code, "expiresAt": expiresAt, "extendedBy": userID})
	return expiresAt, nil
}

func (s *RoomSvc) Leave(userID, code string) error {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return err
	}
	now := s.now().Unix()
	var member model.RoomMember
	if err := s.db.Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.MemberStatusActive).First(&member).Error; err != nil {
		return errx.New(errc.ErrMemberRequired, nil)
	}
	if member.Role == model.MemberRoleOwner {
		return errx.New(errc.ErrOwnerCannotLeave, nil)
	}
	if err := s.db.Model(&model.RoomMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{"status": model.MemberStatusLeft, "left_at": now, "last_seen_at": now}).Error; err != nil {
		return err
	}
	s.publishRoom(room.ID, "room.member_left", map[string]interface{}{"roomCode": room.Code, "userId": userID, "reason": "left"})
	return nil
}

func (s *RoomSvc) Dissolve(userID, code string) error {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return err
	}
	now := s.now()
	result := s.db.Model(&model.Room{}).Where("id = ? AND owner_user_id = ? AND status = ?", room.ID, userID, model.RoomStatusActive).Updates(map[string]interface{}{"status": model.RoomStatusDestroying, "destroying_at": now.Unix(), "destroy_at": now.Add(RoomDestroyDelay).Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errx.New(errc.ErrOwnerRequired, nil)
	}
	s.publishRoom(room.ID, "room.destroying", map[string]interface{}{"roomCode": room.Code, "reason": "owner_dissolved", "destroyAt": now.Add(RoomDestroyDelay).Unix()})
	return nil
}

func (s *RoomSvc) Kick(ownerID, code, targetUserID string) error {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return err
	}
	if room.OwnerUserID != ownerID {
		return errx.New(errc.ErrOwnerRequired, nil)
	}
	if targetUserID == ownerID {
		return errx.New(errc.ErrConflict, nil)
	}
	now := s.now().Unix()
	result := s.db.Model(&model.RoomMember{}).Where("room_id = ? AND user_id = ? AND status = ? AND role = ?", room.ID, targetUserID, model.MemberStatusActive, model.MemberRoleMember).Updates(map[string]interface{}{"status": model.MemberStatusKicked, "kicked_at": now, "kicked_by_user_id": ownerID, "last_seen_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errx.New(errc.ErrMemberRequired, nil)
	}
	s.hub.PublishUser(targetUserID, "room.member_kicked", map[string]interface{}{"roomCode": room.Code, "reason": "kicked"})
	s.publishRoom(room.ID, "room.member_left", map[string]interface{}{"roomCode": room.Code, "userId": targetUserID, "reason": "kicked"})
	return nil
}

func (s *RoomSvc) CreateJoinRequest(userID, code string) (JoinRequestView, error) {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return JoinRequestView{}, err
	}
	if room.JoinMode != model.JoinModeOwnerApproval {
		return JoinRequestView{}, errx.New(errc.ErrJoinModeMismatch, nil)
	}
	now := s.now()
	var result model.JoinRequest
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
			return errx.New(errc.ErrUserNotFound, nil)
		}
		var member model.RoomMember
		if err := tx.Where("room_id = ? AND user_id = ?", room.ID, userID).First(&member).Error; err == nil {
			if member.Status == model.MemberStatusActive {
				return errx.New(errc.ErrDuplicate, nil)
			}
			if member.Status == model.MemberStatusKicked {
				return errx.New(errc.ErrKicked, nil)
			}
		}
		var pending int64
		if err := tx.Model(&model.JoinRequest{}).Where("user_id = ? AND status = ?", userID, model.JoinRequestPending).Count(&pending).Error; err != nil {
			return err
		}
		if pending >= MaxUserPending {
			return errx.New(errc.ErrPendingRequestLimit, nil)
		}
		if err := tx.Model(&model.JoinRequest{}).Where("room_id = ? AND status = ?", room.ID, model.JoinRequestPending).Count(&pending).Error; err != nil {
			return err
		}
		if pending >= MaxRoomPending {
			return errx.New(errc.ErrPendingRequestLimit, nil)
		}
		var latest model.JoinRequest
		if err := tx.Where("room_id = ? AND user_id = ?", room.ID, userID).Order("created_at DESC").First(&latest).Error; err == nil {
			if latest.Status == model.JoinRequestPending {
				return errx.New(errc.ErrPendingRequestExists, nil)
			}
			if latest.NextAllowedAt != nil && *latest.NextAllowedAt > now.Unix() {
				return errx.New(errc.ErrRequestCooldown, nil)
			}
		}
		requestID, err := ulidx.New()
		if err != nil {
			return err
		}
		result = model.JoinRequest{ID: requestID, RoomID: room.ID, UserID: userID, DisplayName: user.DisplayName, Status: model.JoinRequestPending, CreatedAt: now.Unix(), ExpiresAt: now.Add(ApprovalWait).Unix()}
		return tx.Create(&result).Error
	}); err != nil {
		return JoinRequestView{}, err
	}
	view := JoinRequestView{RequestID: result.ID, RoomCode: room.Code, UserID: result.UserID, DisplayName: result.DisplayName, Status: result.Status, CreatedAt: result.CreatedAt, ExpiresAt: result.ExpiresAt}
	s.hub.PublishUser(room.OwnerUserID, "room.join_request_changed", map[string]interface{}{"roomCode": room.Code, "requestId": result.ID, "status": result.Status})
	return view, nil
}

func (s *RoomSvc) ListJoinRequests(ownerID, code string) (int, []JoinRequestView, error) {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return 0, nil, err
	}
	if room.OwnerUserID != ownerID {
		return 0, nil, errx.New(errc.ErrOwnerRequired, nil)
	}
	var requests []model.JoinRequest
	if err := s.db.Where("room_id = ? AND status = ?", room.ID, model.JoinRequestPending).Order("created_at ASC").Find(&requests).Error; err != nil {
		return 0, nil, err
	}
	result := make([]JoinRequestView, 0, len(requests))
	for _, request := range requests {
		result = append(result, JoinRequestView{RequestID: request.ID, RoomCode: room.Code, UserID: request.UserID, DisplayName: request.DisplayName, Status: request.Status, CreatedAt: request.CreatedAt, ExpiresAt: request.ExpiresAt})
	}
	return len(result), result, nil
}

func (s *RoomSvc) CancelJoinRequest(userID, code string) error {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return err
	}
	now := s.now()
	next := now.Add(ApprovalCancelWait).Unix()
	result := s.db.Model(&model.JoinRequest{}).Where("room_id = ? AND user_id = ? AND status = ?", room.ID, userID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestCancelled, "cancelled_at": now.Unix(), "processed_at": now.Unix(), "next_allowed_at": next})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errx.New(errc.ErrJoinRequestNotFound, nil)
	}
	s.hub.PublishUsers([]string{userID, room.OwnerUserID}, "room.join_request_changed", map[string]interface{}{"roomCode": room.Code, "status": model.JoinRequestCancelled})
	return nil
}

func (s *RoomSvc) ApproveJoinRequest(ownerID, code, requestID string) error {
	return s.processJoinRequest(ownerID, code, requestID, true)
}

func (s *RoomSvc) RejectJoinRequest(ownerID, code, requestID string) error {
	return s.processJoinRequest(ownerID, code, requestID, false)
}

func (s *RoomSvc) processJoinRequest(ownerID, code, requestID string, approve bool) error {
	room, err := s.findActiveRoom(code)
	if err != nil {
		return err
	}
	if room.OwnerUserID != ownerID {
		return errx.New(errc.ErrOwnerRequired, nil)
	}
	now := s.now()
	var targetUserID string
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var request model.JoinRequest
		if err := tx.Where("id = ? AND room_id = ? AND status = ?", requestID, room.ID, model.JoinRequestPending).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(errc.ErrJoinRequestResolved, nil)
			}
			return err
		}
		targetUserID = request.UserID
		if request.ExpiresAt <= now.Unix() {
			return errx.New(errc.ErrJoinRequestResolved, nil)
		}
		if !approve {
			next := now.Add(ApprovalRejectWait).Unix()
			return tx.Model(&model.JoinRequest{}).Where("id = ? AND status = ?", requestID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestRejected, "processed_at": now.Unix(), "processed_by_user_id": ownerID, "next_allowed_at": next}).Error
		}
		var members int64
		if err := tx.Model(&model.RoomMember{}).Where("room_id = ? AND status = ?", room.ID, model.MemberStatusActive).Count(&members).Error; err != nil {
			return err
		}
		if members >= MaxRoomMembers {
			return errx.New(errc.ErrRoomFull, nil)
		}
		var joined int64
		if err := tx.Model(&model.RoomMember{}).Where("user_id = ? AND role = ? AND status = ?", request.UserID, model.MemberRoleMember, model.MemberStatusActive).Count(&joined).Error; err != nil {
			return err
		}
		if joined >= MaxJoinedRooms {
			return errx.New(errc.ErrJoinedRoomLimit, nil)
		}
		var member model.RoomMember
		memberErr := tx.Where("room_id = ? AND user_id = ?", room.ID, request.UserID).First(&member).Error
		if memberErr == nil && member.Status == model.MemberStatusKicked {
			return errx.New(errc.ErrKicked, nil)
		}
		if memberErr == nil {
			if err := tx.Model(&model.RoomMember{}).Where("id = ?", member.ID).Updates(map[string]interface{}{"status": model.MemberStatusActive, "display_name": request.DisplayName, "joined_at": now.Unix(), "last_seen_at": now.Unix(), "left_at": nil}).Error; err != nil {
				return err
			}
		} else if errors.Is(memberErr, gorm.ErrRecordNotFound) {
			memberID, err := ulidx.New()
			if err != nil {
				return err
			}
			if err := tx.Create(&model.RoomMember{ID: memberID, RoomID: room.ID, UserID: request.UserID, Role: model.MemberRoleMember, DisplayName: request.DisplayName, Status: model.MemberStatusActive, JoinedAt: now.Unix(), LastSeenAt: now.Unix()}).Error; err != nil {
				return err
			}
		} else {
			return memberErr
		}
		return tx.Model(&model.JoinRequest{}).Where("id = ? AND status = ?", requestID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestApproved, "processed_at": now.Unix(), "processed_by_user_id": ownerID}).Error
	}); err != nil {
		return err
	}
	status := model.JoinRequestRejected
	if approve {
		status = model.JoinRequestApproved
		s.publishRoom(room.ID, "room.member_joined", map[string]interface{}{"roomCode": room.Code, "userId": targetUserID, "role": model.MemberRoleMember})
	}
	s.hub.PublishUser(targetUserID, "room.join_request_changed", map[string]interface{}{"roomCode": room.Code, "requestId": requestID, "status": status})
	s.hub.PublishUser(ownerID, "room.join_request_changed", map[string]interface{}{"roomCode": room.Code, "requestId": requestID, "status": status})
	return nil
}

func (s *RoomSvc) publishRoom(roomID, eventType string, payload interface{}) {
	if s == nil || s.hub == nil || roomID == "" {
		return
	}
	var members []model.RoomMember
	if err := s.db.Where("room_id = ? AND status = ?", roomID, model.MemberStatusActive).Find(&members).Error; err != nil {
		return
	}
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	s.hub.PublishUsers(userIDs, eventType, payload)
}

func (s *RoomSvc) findRoom(code string) (model.Room, error) {
	code = strings.TrimSpace(code)
	if len(code) != 4 {
		return model.Room{}, errx.New(errc.ErrRoomCode, nil)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return model.Room{}, errx.New(errc.ErrRoomCode, nil)
		}
	}
	var room model.Room
	if err := s.db.Where("code = ?", code).Order("created_at DESC").First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Room{}, errx.New(errc.ErrRoomNotFound, nil)
		}
		return model.Room{}, err
	}
	return room, nil
}

func (s *RoomSvc) findActiveRoom(code string) (model.Room, error) {
	room, err := s.findRoom(code)
	if err != nil {
		return model.Room{}, err
	}
	if room.Status != model.RoomStatusActive {
		return model.Room{}, errx.New(errc.ErrRoomNotFound, nil)
	}
	return room, nil
}

func validatePIN(pin string) error {
	if len(pin) != 4 {
		return errx.New(errc.ErrPIN, nil)
	}
	for _, r := range pin {
		if !unicode.IsDigit(r) || r > unicode.MaxASCII {
			return errx.New(errc.ErrPIN, nil)
		}
	}
	return nil
}
