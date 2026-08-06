package service

import (
	"testing"

	"github.com/leafney/filedock/internal/model"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func TestFileNotificationRecorderIsIdempotentPerDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.NotificationRecord{}); err != nil {
		t.Fatalf("migrate notification record: %v", err)
	}
	recorder, err := NewFileNotificationRecorder(db)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	input := FileNotificationRecordInput{
		UserID:            "recipient",
		Type:              NotificationTypeFileReceived,
		RoomID:            "room",
		FileID:            "file",
		CounterpartUserID: "sender",
		FileRecipientID:   "relation",
		DeliveryVersion:   1,
		SourceEventID:     "event-one",
		OccurredAtMS:      1_000,
	}
	created, err := recorder.Record(nil, input)
	if err != nil || !created {
		t.Fatalf("record first = %v, %v", created, err)
	}
	created, err = recorder.Record(nil, input)
	if err != nil || created {
		t.Fatalf("record duplicate = %v, %v", created, err)
	}

	input.DeliveryVersion = 2
	input.SourceEventID = "event-two"
	input.OccurredAtMS = 2_000
	created, err = recorder.Record(nil, input)
	if err != nil || !created {
		t.Fatalf("record new delivery = %v, %v", created, err)
	}
	input.Type = NotificationTypeFileDeclined
	input.UserID = "sender"
	input.CounterpartUserID = "recipient"
	created, err = recorder.Record(nil, input)
	if err != nil || !created {
		t.Fatalf("record result type = %v, %v", created, err)
	}

	var count int64
	if err := db.Model(&model.NotificationRecord{}).Count(&count).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 3 {
		t.Fatalf("record count = %d, want 3", count)
	}
}

func TestFileNotificationRecorderRejectsIncompleteInput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	recorder, err := NewFileNotificationRecorder(db)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	if _, err := recorder.Record(nil, FileNotificationRecordInput{Type: NotificationTypeFileReceived}); err == nil {
		t.Fatal("incomplete input unexpectedly succeeded")
	}
}

func TestFileNotificationRecorderSupportsTrashAndRequestOccurrences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.NotificationRecord{}); err != nil {
		t.Fatal(err)
	}
	recorder, err := NewFileNotificationRecorder(db)
	if err != nil {
		t.Fatal(err)
	}
	base := FileNotificationRecordInput{UserID: "uploader", RoomID: "room", FileID: "file", CounterpartUserID: "owner", TrashCycleID: "cycle", TrashVersion: 1, SourceEventID: "event", OccurredAtMS: 1_000}
	base.Type = NotificationTypeFileTrashedByOwner
	if created, err := recorder.Record(nil, base); err != nil || !created {
		t.Fatalf("trash record=%v error=%v", created, err)
	}
	if created, err := recorder.Record(nil, base); err != nil || created {
		t.Fatalf("duplicate trash record=%v error=%v", created, err)
	}
	base.Type = NotificationTypeFileRestoreRequested
	base.UserID = "owner"
	base.CounterpartUserID = "uploader"
	base.RestoreRequestID = "request"
	base.SourceEventID = "request-event"
	if created, err := recorder.Record(nil, base); err != nil || !created {
		t.Fatalf("request record=%v error=%v", created, err)
	}
}
