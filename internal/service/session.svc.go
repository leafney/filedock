package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/passwordx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	SessionCookieName  = "session_token"
	SessionContextKey  = "filedock.session_principal"
	SessionIssuer      = "filedock"
	SessionSubject     = "anonymous_session"
	SessionTTL         = 30 * 24 * time.Hour
	SessionRenewWindow = 7 * 24 * time.Hour
)

type sessionClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

type Principal struct {
	UserID           string
	SessionID        string
	DisplayName      string
	CreatedAt        int64
	SessionExpiresAt int64
	RefreshedToken   string
}

type SessionResult struct {
	Principal Principal
	Token     string
}

type SessionSvc struct {
	db       *gorm.DB
	nickname *NicknameSvc
	hub      *StreamHub
	key      []byte
	now      func() time.Time
}

func NewSessionSvc(db *gorm.DB, dataDir string, nickname *NicknameSvc, hubs ...*StreamHub) (*SessionSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("session database is required")
	}
	if nickname == nil {
		return nil, fmt.Errorf("session nickname service is required")
	}
	now := time.Now
	key, err := loadSigningKey(db, dataDir, now)
	if err != nil {
		return nil, err
	}
	var hub *StreamHub
	if len(hubs) > 0 {
		hub = hubs[0]
	}
	return &SessionSvc{db: db, nickname: nickname, hub: hub, key: key, now: now}, nil
}

func loadSigningKey(db *gorm.DB, dataDir string, now func() time.Time) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("jwt key database is required")
	}
	keyDir := filepath.Join(dataDir, "keys")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return nil, fmt.Errorf("create jwt key directory: %w", err)
	}
	_ = os.Chmod(keyDir, 0o700)
	keyPath := filepath.Join(keyDir, "jwt_signing.key")
	if _, statErr := os.Stat(keyPath); errors.Is(statErr, os.ErrNotExist) {
		var activeSessions int64
		if err := db.Model(&model.Session{}).Where("revoked_at IS NULL AND expires_at > ?", now().Unix()).Count(&activeSessions).Error; err != nil {
			return nil, fmt.Errorf("check active sessions for jwt key: %w", err)
		}
		if activeSessions > 0 {
			return nil, fmt.Errorf("jwt signing key is missing while active sessions exist")
		}
	} else if statErr != nil {
		return nil, fmt.Errorf("stat jwt signing key: %w", statErr)
	}
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		key := make([]byte, 32)
		if _, readErr := rand.Read(key); readErr != nil {
			_ = keyFile.Close()
			_ = os.Remove(keyPath)
			return nil, fmt.Errorf("generate jwt signing key: %w", readErr)
		}
		if _, writeErr := keyFile.Write(key); writeErr != nil {
			_ = keyFile.Close()
			_ = os.Remove(keyPath)
			return nil, fmt.Errorf("write jwt signing key: %w", writeErr)
		}
		if syncErr := keyFile.Sync(); syncErr != nil {
			_ = keyFile.Close()
			return nil, fmt.Errorf("sync jwt signing key: %w", syncErr)
		}
		if closeErr := keyFile.Close(); closeErr != nil {
			return nil, fmt.Errorf("close jwt signing key: %w", closeErr)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("open jwt signing key: %w", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		return nil, fmt.Errorf("stat jwt signing key: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("jwt signing key permissions are too broad")
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read jwt signing key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("jwt signing key must contain 32 bytes")
	}
	return key, nil
}

func (s *SessionSvc) Create(displayName, deviceHint string) (SessionResult, error) {
	if s == nil || s.db == nil || s.nickname == nil {
		return SessionResult{}, fmt.Errorf("session service is nil")
	}
	now := s.now()
	expiresAt := now.Add(SessionTTL)
	userID, err := ulidx.New()
	if err != nil {
		return SessionResult{}, fmt.Errorf("generate user id: %w", err)
	}
	sessionID, err := ulidx.New()
	if err != nil {
		return SessionResult{}, fmt.Errorf("generate session id: %w", err)
	}
	jwtID, err := ulidx.New()
	if err != nil {
		return SessionResult{}, fmt.Errorf("generate jwt id: %w", err)
	}
	displayName, nameKey, err := NormalizeNickname(displayName)
	if err != nil {
		return SessionResult{}, err
	}
	claims := s.claims(userID, sessionID, jwtID, expiresAt)
	token, err := s.sign(claims)
	if err != nil {
		return SessionResult{}, fmt.Errorf("sign session token: %w", err)
	}
	user := model.User{
		ID:                userID,
		DisplayName:       displayName,
		DisplayNameKey:    nameKey,
		Status:            model.UserStatusActive,
		DeviceHint:        deviceHint,
		CreatedAt:         now.Unix(),
		UpdatedAt:         now.Unix(),
		LastSeenAt:        now.Unix(),
		IdentityExpiresAt: expiresAt.Unix(),
	}
	session := model.Session{
		ID:             sessionID,
		UserID:         userID,
		JWTID:          jwtID,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
		LastActivityAt: now.Unix(),
		ExpiresAt:      expiresAt.Unix(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.User{}).Where("display_name_key = ? AND status = ?", nameKey, model.UserStatusActive).Count(&count).Error; err != nil {
			return fmt.Errorf("check nickname uniqueness: %w", err)
		}
		if count > 0 {
			return errx.New(errc.ErrNicknameTaken, nil)
		}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := tx.Create(&session).Error; err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		return nil
	}); err != nil {
		return SessionResult{}, err
	}
	return SessionResult{Principal: Principal{UserID: userID, SessionID: sessionID, DisplayName: displayName, CreatedAt: now.Unix(), SessionExpiresAt: expiresAt.Unix()}, Token: token}, nil
}

func (s *SessionSvc) Authenticate(token string) (Principal, error) {
	if s == nil || s.db == nil {
		return Principal{}, fmt.Errorf("session service is nil")
	}
	if token == "" {
		return Principal{}, errx.New(errc.ErrUnAuthorized, nil)
	}
	claims := &sessionClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(parsedToken *jwt.Token) (interface{}, error) {
		if parsedToken.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected jwt signing method")
		}
		return s.key, nil
	}, jwt.WithIssuer(SessionIssuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Principal{}, errx.New(errc.ErrAuthExpired, nil)
		}
		return Principal{}, errx.Wrap(errc.ErrTokenInvalid, err, nil)
	}
	if !parsed.Valid || claims.Subject != SessionSubject || claims.UserID == "" || claims.SessionID == "" || claims.ID == "" || claims.ExpiresAt == nil {
		return Principal{}, errx.New(errc.ErrTokenInvalid, nil)
	}
	now := s.now()
	var session model.Session
	if err := s.db.Where("id = ?", claims.SessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Principal{}, errx.New(errc.ErrTokenPermissionOrTimeout, nil)
		}
		return Principal{}, fmt.Errorf("load session: %w", err)
	}
	if session.UserID != claims.UserID || session.JWTID != claims.ID || session.RevokedAt != nil || session.ExpiresAt <= now.Unix() {
		return Principal{}, errx.New(errc.ErrTokenPermissionOrTimeout, nil)
	}
	var user model.User
	if err := s.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Principal{}, errx.New(errc.ErrUserNotFound, nil)
		}
		return Principal{}, fmt.Errorf("load session user: %w", err)
	}
	if user.Status != model.UserStatusActive || user.IdentityExpiresAt <= now.Unix() {
		return Principal{}, errx.New(errc.ErrAuthExpired, nil)
	}
	principal := Principal{UserID: user.ID, SessionID: session.ID, DisplayName: user.DisplayName, CreatedAt: user.CreatedAt, SessionExpiresAt: session.ExpiresAt}
	if time.Until(time.Unix(session.ExpiresAt, 0)) <= SessionRenewWindow {
		if err := s.renew(&session, &principal, now); err != nil {
			return Principal{}, err
		}
	}
	if now.Unix()-session.LastActivityAt >= 60 {
		s.db.Model(&model.Session{}).Where("id = ?", session.ID).Updates(map[string]interface{}{"last_activity_at": now.Unix(), "updated_at": now.Unix()})
		s.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{"last_seen_at": now.Unix(), "updated_at": now.Unix()})
	}
	return principal, nil
}

func (s *SessionSvc) renew(session *model.Session, principal *Principal, now time.Time) error {
	newJWTID, err := ulidx.New()
	if err != nil {
		return fmt.Errorf("generate renewed jwt id: %w", err)
	}
	expiresAt := now.Add(SessionTTL)
	token, err := s.sign(s.claims(principal.UserID, session.ID, newJWTID, expiresAt))
	if err != nil {
		return fmt.Errorf("sign renewed session token: %w", err)
	}
	if err := s.db.Model(&model.Session{}).Where("id = ? AND jwt_id = ? AND revoked_at IS NULL", session.ID, session.JWTID).Updates(map[string]interface{}{"jwt_id": newJWTID, "expires_at": expiresAt.Unix(), "updated_at": now.Unix(), "last_activity_at": now.Unix()}).Error; err != nil {
		return fmt.Errorf("renew session: %w", err)
	}
	s.db.Model(&model.User{}).Where("id = ?", principal.UserID).Updates(map[string]interface{}{"identity_expires_at": expiresAt.Unix(), "last_seen_at": now.Unix(), "updated_at": now.Unix()})
	principal.SessionExpiresAt = expiresAt.Unix()
	principal.RefreshedToken = token
	return nil
}

func (s *SessionSvc) UpdateName(userID, displayName string) (Principal, error) {
	if s == nil || s.db == nil || s.nickname == nil {
		return Principal{}, fmt.Errorf("session service is nil")
	}
	now := s.now().Unix()
	displayName, key, err := NormalizeNickname(displayName)
	if err != nil {
		return Principal{}, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(errc.ErrUserNotFound, nil)
			}
			return err
		}
		if user.DisplayNameKey != key {
			var count int64
			if err := tx.Model(&model.User{}).Where("display_name_key = ? AND status = ? AND id <> ?", key, model.UserStatusActive, userID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errx.New(errc.ErrNicknameTaken, nil)
			}
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{"display_name": displayName, "display_name_key": key, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.RoomMember{}).Where("user_id = ? AND status = ?", userID, model.MemberStatusActive).Updates(map[string]interface{}{"display_name": displayName}).Error
	}); err != nil {
		return Principal{}, err
	}
	if s.hub != nil {
		s.publishProfileChanged(userID, displayName)
	}
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return Principal{}, err
	}
	return Principal{UserID: user.ID, DisplayName: user.DisplayName, CreatedAt: user.CreatedAt, SessionExpiresAt: user.IdentityExpiresAt}, nil
}

func (s *SessionSvc) Reset(userID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session service is nil")
	}
	now := s.now().Unix()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var owned int64
		if err := tx.Model(&model.Room{}).Where("owner_user_id = ? AND status IN ?", userID, []string{model.RoomStatusActive, model.RoomStatusDestroying}).Count(&owned).Error; err != nil {
			return err
		}
		if owned > 0 {
			return errx.New(errc.ErrIdentityResetBlocked, nil)
		}
		if err := tx.Model(&model.RoomMember{}).Where("user_id = ? AND status = ?", userID, model.MemberStatusActive).Updates(map[string]interface{}{"status": model.MemberStatusLeft, "left_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.JoinRequest{}).Where("user_id = ? AND status = ?", userID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestCancelled, "cancelled_at": now, "processed_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Session{}).Where("user_id = ? AND revoked_at IS NULL", userID).Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{"status": model.UserStatusRevoked, "display_name_key": "", "identity_expires_at": now, "updated_at": now}).Error
	}); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.PublishUser(userID, "session.revoked", map[string]interface{}{"reason": "reset"})
		s.hub.CloseUser(userID)
	}
	return nil
}

func (s *SessionSvc) publishProfileChanged(userID, displayName string) {
	var memberships []model.RoomMember
	if err := s.db.Where("user_id = ? AND status = ?", userID, model.MemberStatusActive).Find(&memberships).Error; err != nil {
		return
	}
	for _, membership := range memberships {
		var members []model.RoomMember
		if err := s.db.Where("room_id = ? AND status = ?", membership.RoomID, model.MemberStatusActive).Find(&members).Error; err != nil {
			continue
		}
		userIDs := make([]string, 0, len(members))
		for _, member := range members {
			userIDs = append(userIDs, member.UserID)
		}
		s.hub.PublishUsers(userIDs, "user.profile_changed", map[string]interface{}{"userId": userID, "displayName": displayName})
	}
}

func (s *SessionSvc) claims(userID, sessionID, jwtID string, expiresAt time.Time) *sessionClaims {
	now := s.now()
	return &sessionClaims{UserID: userID, SessionID: sessionID, RegisteredClaims: jwt.RegisteredClaims{ID: jwtID, Issuer: SessionIssuer, Subject: SessionSubject, IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt)}}
}

func (s *SessionSvc) sign(claims *sessionClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.key)
}

func (s *SessionSvc) TokenTTL() time.Duration { return SessionTTL }

func (s *SessionSvc) ComparePIN(hash, pin string) error { return passwordx.Compare(hash, pin) }
