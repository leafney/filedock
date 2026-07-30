package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leafney/filedock/internal/dal"
	"github.com/leafney/filedock/internal/model"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func newSessionTestService(t *testing.T) (*SessionSvc, *gorm.DB, string) {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dal.AutoMigrate(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	nickname, err := NewNicknameSvc(db)
	if err != nil {
		t.Fatalf("new nickname service: %v", err)
	}
	dataDir := t.TempDir()
	svc, err := NewSessionSvc(db, dataDir, nickname)
	if err != nil {
		t.Fatalf("new session service: %v", err)
	}
	return svc, db, filepath.Join(dataDir, "keys", "jwt_signing.key")
}

func TestSessionCreateAndAuthenticate(t *testing.T) {
	svc, _, _ := newSessionTestService(t)
	result, err := svc.Create("张飞", "Chrome")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Token == "" || result.Principal.UserID == "" || result.Principal.SessionExpiresAt <= 0 {
		t.Fatalf("Create() returned incomplete result: %+v", result)
	}
	principal, err := svc.Authenticate(result.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.UserID != result.Principal.UserID || principal.DisplayName != "张飞" {
		t.Fatalf("Authenticate() principal = %+v, want user %s and name 张飞", principal, result.Principal.UserID)
	}
}

func TestSessionRejectsDuplicateNickname(t *testing.T) {
	svc, _, _ := newSessionTestService(t)
	if _, err := svc.Create("张飞", ""); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if _, err := svc.Create(" 张飞 ", ""); err == nil {
		t.Fatal("second Create() unexpectedly succeeded")
	}
}

func TestSessionMissingKeyFailsWhenActiveSessionsExist(t *testing.T) {
	svc, _, keyPath := newSessionTestService(t)
	if _, err := svc.Create("关羽", ""); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove key: %v", err)
	}
	nickname, err := NewNicknameSvc(svc.db)
	if err != nil {
		t.Fatalf("new nickname service: %v", err)
	}
	if _, err := NewSessionSvc(svc.db, filepath.Dir(filepath.Dir(keyPath)), nickname); err == nil {
		t.Fatal("NewSessionSvc() unexpectedly succeeded with missing key and active session")
	}
}

func TestSessionResetBlockedByOwnedRoom(t *testing.T) {
	svc, db, _ := newSessionTestService(t)
	result, err := svc.Create("刘备", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.Create(&model.Room{ID: "01ROOM", Code: "0001", Title: "房间 0001", OwnerUserID: result.Principal.UserID, JoinMode: model.JoinModeOpen, Status: model.RoomStatusActive, CreatedAt: 1, ExpiresAt: 2}).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := svc.Reset(result.Principal.UserID); err == nil {
		t.Fatal("Reset() unexpectedly succeeded while owning a room")
	}
}
