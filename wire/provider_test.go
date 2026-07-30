package wire

import (
	"path/filepath"
	"testing"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/pkg/zlogx"
)

func TestProvideSQLiteDB(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.App.DataDir = dir
	cfg.SQLite.Path = filepath.Join(dir, "filedock.db")
	log := zlogx.NewZLogSvcWithConfig(zlogx.Config{Enable: false, Level: "info", Output: "stdout"}, nil)

	db, err := provideSQLiteDB(cfg, log)
	if err != nil {
		t.Fatalf("provideSQLiteDB() error = %v", err)
	}
	if db == nil {
		t.Fatal("provideSQLiteDB() returned nil database")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestSQLiteDSNAndPragmas(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.App.DataDir = dir
	cfg.SQLite.Path = filepath.Join(dir, "filedock.db")
	log := zlogx.NewZLogSvcWithConfig(zlogx.Config{Enable: false, Level: "info", Output: "stdout"}, nil)

	db, err := provideSQLiteDB(cfg, log)
	if err != nil {
		t.Fatalf("provideSQLiteDB() error = %v", err)
	}
	defer db.Close()

	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}

	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("journal_mode query error = %v", err)
	}
	if journalMode != "wal" && journalMode != "WAL" {
		t.Fatalf("journal_mode = %q, want WAL", journalMode)
	}

	var foreignKeys int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		t.Fatalf("foreign_keys query error = %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatalf("busy_timeout query error = %v", err)
	}
	if busyTimeout < sqliteBusyTimeout {
		t.Fatalf("busy_timeout = %d, want at least %d", busyTimeout, sqliteBusyTimeout)
	}

	var tables int
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tables).Error; err != nil {
		t.Fatalf("table query error = %v", err)
	}
	if tables != 6 {
		t.Fatalf("business table count = %d, want 6", tables)
	}
}

func TestBuildSQLiteDSN(t *testing.T) {
	path := "/tmp/filedock.db"
	if got := buildSQLiteDSN(path); got == path {
		t.Fatal("buildSQLiteDSN() did not append pragmas")
	}
	if got := buildSQLiteDSN(path + "?mode=rwc"); got == path+"?mode=rwc?" {
		t.Fatal("buildSQLiteDSN() used duplicate query separator")
	}
}
