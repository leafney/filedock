package dal

import (
	"testing"

	"github.com/leafney/filedock/internal/model"
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
	for _, name := range []string{"users", "sessions", "rooms", "room_members", "join_requests", "cleanup_jobs", "upload_batches", "room_files", "file_trash_cycles", "file_restore_requests", "file_recipients", "file_events", "download_tasks", "notification_records", "chat_conversations", "chat_messages", "chat_read_states", "chat_message_deletions"} {
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
	if !db.Migrator().HasColumn("file_recipients", "delivery_version") {
		t.Fatal("file recipient delivery version was not migrated")
	}
	if !db.Migrator().HasColumn("room_files", "trash_version") || !db.Migrator().HasColumn("download_tasks", "cancelled_at") {
		t.Fatal("file trash lifecycle columns were not migrated")
	}
	if !db.Migrator().HasColumn("notification_records", "occurrence_key") || !db.Migrator().HasColumn("notification_records", "trash_cycle_id") || !db.Migrator().HasColumn("notification_records", "restore_request_id") {
		t.Fatal("file trash notification columns were not migrated")
	}
}

func TestChatSchemaConstraints(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	conversation := model.ChatConversation{ID: "01CHATCONVERSATION01", RoomID: "01CHATROOM0000000000000001", UserAID: "01CHATA000000000000000001", UserBID: "01CHATB000000000000000001", CreatedAt: 1, UpdatedAt: 1}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	duplicateConversation := model.ChatConversation{ID: "01CHATCONVERSATION02", RoomID: conversation.RoomID, UserAID: conversation.UserAID, UserBID: conversation.UserBID, CreatedAt: 1, UpdatedAt: 1}
	if err := db.Create(&duplicateConversation).Error; err == nil {
		t.Fatal("duplicate conversation unexpectedly succeeded")
	}
	message := model.ChatMessage{ID: "01CHATMESSAGE000000000000001", RoomID: conversation.RoomID, ConversationID: conversation.ID, SenderUserID: conversation.UserAID, RecipientUserID: conversation.UserBID, SenderDisplayName: "甲", ClientMessageID: "client-1", ContentText: "hello", Sequence: 1, CreatedAt: 1}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	duplicateMessage := message
	duplicateMessage.ID = "01CHATMESSAGE000000000000002"
	if err := db.Create(&duplicateMessage).Error; err == nil {
		t.Fatal("duplicate client message unexpectedly succeeded")
	}
}
