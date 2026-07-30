package dal

import (
	"fmt"

	"github.com/leafney/filedock/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate creates the tables required by the current application stage.
// Future file and chat models should be appended here when their stage starts.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	return db.AutoMigrate(
		&model.User{},
		&model.Session{},
		&model.Room{},
		&model.RoomMember{},
		&model.JoinRequest{},
		&model.CleanupJob{},
	)
}
