package gormx

import (
	"testing"

	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func TestNewGormDBSvcWithSQLiteDialector(t *testing.T) {
	log := zlogx.NewZLogSvcWithConfig(zlogx.Config{Enable: false}, nil)
	var called bool

	db, err := NewGormDBSvc(Options{
		Dialector: sqlite.Open("file::memory:?cache=shared"),
		Retry: RetryOptions{
			Enabled: false,
		},
		OnOpen: func(gdb *gorm.DB) error {
			called = true
			return gdb.Exec("PRAGMA journal_mode=WAL;").Error
		},
	}, log)
	if err != nil {
		t.Fatalf("NewGormDBSvc() error = %v", err)
	}
	if !called {
		t.Fatal("expected OnOpen to be called")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
