package service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
)

func TestPrivateFileStreamUsesPerViewerSafeProjection(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	file := makeAvailableFile(t, fixture, "stream-private", model.FileScopeDirect, "stream-secret.pdf", 100, []string{fixture.recipient.UserID})
	hub := NewStreamHub()
	fixture.svc.hub = hub
	uploaderEvents, stopUploader := hub.Subscribe(fixture.uploader.UserID)
	recipientEvents, stopRecipient := hub.Subscribe(fixture.recipient.UserID)
	ownerEvents, stopOwner := hub.Subscribe(fixture.owner.UserID)
	outsiderEvents, stopOutsider := hub.Subscribe(fixture.outsider.UserID)
	defer stopUploader()
	defer stopRecipient()
	defer stopOwner()
	defer stopOutsider()

	fixture.svc.publishFileProjection(file.ID, "file.test", map[string]interface{}{"progress": 50})
	for name, channel := range map[string]<-chan *StreamEvent{"uploader": uploaderEvents, "recipient": recipientEvents, "owner": ownerEvents} {
		select {
		case event := <-channel:
			payload, ok := event.Payload.(map[string]interface{})
			if !ok {
				t.Fatalf("%s payload type=%T", name, event.Payload)
			}
			projection, ok := payload["file"].(FileProjection)
			if !ok {
				t.Fatalf("%s file projection type=%T", name, payload["file"])
			}
			if name == "owner" {
				if projection.Level != ProjectionAnonymous || projection.OriginalName != "" || projection.DeclaredMIME != "" {
					t.Fatalf("owner projection leaked: %+v", projection)
				}
			} else if projection.Level != ProjectionFull || projection.OriginalName != "stream-secret.pdf" {
				t.Fatalf("%s projection=%+v", name, projection)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive event", name)
		}
	}
	select {
	case event := <-outsiderEvents:
		t.Fatalf("outsider received private event: %+v", event)
	default:
	}
}

func TestRoomSnapshotCapacityDetailsAreOwnerOnly(t *testing.T) {
	fixture := newFileTestFixture(t, 10_000)
	makeAvailableFile(t, fixture, "capacity-shared", model.FileScopeShared, "shared", 100, nil)
	makeAvailableFile(t, fixture, "capacity-direct", model.FileScopeDirect, "direct", 200, []string{fixture.recipient.UserID})
	if _, err := fixture.svc.CreateUploadBatch(fixture.uploader.UserID, fixture.room.Code, "capacity-reserved", model.FileScopeShared, []FileManifest{{OriginalName: "reserved", DeclaredSize: 50}}, nil); err != nil {
		t.Fatal(err)
	}
	roomService, err := NewRoomSvc(fixture.svc.db, NewStreamHub())
	if err != nil {
		t.Fatal(err)
	}
	owner, err := roomService.Snapshot(fixture.owner.UserID, fixture.room.Code)
	if err != nil {
		t.Fatal(err)
	}
	if owner.Capacity.UsedBytes != 300 || owner.Capacity.CapacityBytes != 10_000 || owner.Capacity.SharedBytes == nil || *owner.Capacity.SharedBytes != 100 || owner.Capacity.DirectBytes == nil || *owner.Capacity.DirectBytes != 200 || owner.Capacity.ReservedBytes == nil || *owner.Capacity.ReservedBytes != 50 {
		t.Fatalf("owner capacity=%+v", owner.Capacity)
	}
	member, err := roomService.Snapshot(fixture.recipient.UserID, fixture.room.Code)
	if err != nil {
		t.Fatal(err)
	}
	if member.Capacity.UsedBytes != 300 || member.Capacity.CapacityBytes != 10_000 || member.Capacity.SharedBytes != nil || member.Capacity.DirectBytes != nil || member.Capacity.ReservedBytes != nil {
		t.Fatalf("member capacity leaked details: %+v", member.Capacity)
	}
}

func TestCancellingMemberTransfersStopsActiveDownload(t *testing.T) {
	fixture := newFileTestFixture(t, 1000)
	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := makeStoredAvailableFile(t, fixture, storage, "cancel-member-download", model.FileScopeShared, "cancel.txt", []byte("cancel"), nil)
	task, err := fixture.svc.CreateDownloadTask(fixture.recipient.UserID, fixture.room.Code, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := fixture.svc.BeginDownload(fixture.recipient.UserID, fixture.room.Code, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.svc.CancelMemberTransfers(fixture.room.ID, fixture.recipient.UserID)
	if err := stream.WriteTo(context.Background(), &bytes.Buffer{}); err == nil {
		t.Fatal("cancelled member download unexpectedly succeeded")
	}
	var storedTask model.DownloadTask
	fixture.svc.db.First(&storedTask, "id = ?", task.TaskID)
	if storedTask.Status != model.DownloadTaskFailed {
		t.Fatalf("cancelled task status=%q", storedTask.Status)
	}
}
