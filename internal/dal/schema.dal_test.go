package dal

import (
	"testing"

	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateCreatesCurrentStageTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	for _, name := range []string{"users", "sessions", "rooms", "room_members", "join_requests", "cleanup_jobs", "upload_batches", "room_files", "file_recipients", "file_events", "download_tasks"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
			t.Fatalf("check %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", name, count)
		}
	}
	if !db.Migrator().HasColumn("rooms", "capacity_bytes") || !db.Migrator().HasColumn("rooms", "used_bytes") || !db.Migrator().HasColumn("rooms", "reserved_bytes") {
		t.Fatal("rooms capacity columns were not migrated")
	}
}
