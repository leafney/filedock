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
	for _, name := range []string{"users", "sessions", "rooms", "room_members", "join_requests", "cleanup_jobs"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
			t.Fatalf("check %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", name, count)
		}
	}
}
