package dal

import (
	"fmt"

	"github.com/leafney/filedock/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate creates the tables required by the current application stage.
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
		&model.UploadBatch{},
		&model.RoomFile{},
		&model.FileRecipient{},
		&model.FileEvent{},
		&model.DownloadTask{},
	)
}
