package service

import (
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
)

func TestRoomCodeRemainsLockedWhenCleanupFails(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	reusableAt := time.Now().Add(-time.Minute).Unix()
	if err := db.Create(&model.Room{ID: "01ROOMFAILEDLOCK01", Code: "0001", Title: "房间 0001", OwnerUserID: "01USERFAILEDLOCK01", JoinMode: model.JoinModeOpen, Status: model.RoomStatusDestroyed, CreatedAt: 1, ExpiresAt: 2, CodeReusableAt: &reusableAt}).Error; err != nil {
		t.Fatalf("create failed room: %v", err)
	}
	if err := db.Create(&model.CleanupJob{ID: "01JOBFAILEDLOCK001", RoomID: "01ROOMFAILEDLOCK01", Status: model.CleanupFailed, Phase: "failed", ScheduledAt: 1, AttemptCount: 2}).Error; err != nil {
		t.Fatalf("create failed cleanup job: %v", err)
	}
	available, err := roomCodeAvailable(db, "0001", time.Now().Unix())
	if err != nil {
		t.Fatalf("roomCodeAvailable() error = %v", err)
	}
	if available {
		t.Fatal("room code became available while cleanup failed")
	}
}

func TestRoomCreateAndOpenJoin(t *testing.T) {
	session, _, _ := newSessionTestService(t)
	owner, err := session.Create("刘备", "")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	guest, err := session.Create("关羽", "")
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	room, err := NewRoomSvc(session.db, nil)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	snapshot, err := room.Create(owner.Principal.UserID, model.JoinModeOpen, "", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if snapshot.RoomCode < "0001" || snapshot.RoomCode > "9999" || snapshot.Role != model.MemberRoleOwner || snapshot.Title != "房间 "+snapshot.RoomCode {
		t.Fatalf("unexpected room snapshot: %+v", snapshot)
	}
	if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, false, ""); err == nil {
		t.Fatal("open join without confirmation unexpectedly succeeded")
	}
	joined, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, "")
	if err != nil {
		t.Fatalf("open join: %v", err)
	}
	if repeated, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, false, ""); err != nil || repeated.Role != model.MemberRoleMember {
		t.Fatalf("repeated open join should be idempotent, snapshot=%+v error=%v", repeated, err)
	}
	if joined.Role != model.MemberRoleMember || len(joined.Members) != 2 {
		t.Fatalf("unexpected joined snapshot: %+v", joined)
	}
	if err := room.Leave(owner.Principal.UserID, snapshot.RoomCode); err == nil {
		t.Fatal("owner leave unexpectedly succeeded")
	}
}

func TestRoomLeavePublishesMemberDetails(t *testing.T) {
	session, _, _ := newSessionTestService(t)
	owner, err := session.Create("刘备", "")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	guest, err := session.Create("关羽", "")
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	hub := NewStreamHub()
	room, err := NewRoomSvc(session.db, hub)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	snapshot, err := room.Create(owner.Principal.UserID, model.JoinModeOpen, "", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, ""); err != nil {
		t.Fatalf("join guest: %v", err)
	}
	ownerEvents, unsubscribe := hub.Subscribe(owner.Principal.UserID)
	defer unsubscribe()

	if err := room.Leave(guest.Principal.UserID, snapshot.RoomCode); err != nil {
		t.Fatalf("leave room: %v", err)
	}

	select {
	case event := <-ownerEvents:
		if event.Type != "room.member_left" {
			t.Fatalf("event type = %q", event.Type)
		}
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("payload type = %T", event.Payload)
		}
		if payload["userId"] != guest.Principal.UserID || payload["displayName"] != guest.Principal.DisplayName || payload["reason"] != "left" {
			t.Fatalf("event payload = %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("owner did not receive member-left event")
	}
}

func TestRoomJoinPublishesMemberDetails(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		session, _, _ := newSessionTestService(t)
		owner, err := session.Create("刘备", "")
		if err != nil {
			t.Fatalf("create owner: %v", err)
		}
		guest, err := session.Create("关羽", "")
		if err != nil {
			t.Fatalf("create guest: %v", err)
		}
		hub := NewStreamHub()
		room, err := NewRoomSvc(session.db, hub)
		if err != nil {
			t.Fatalf("new room service: %v", err)
		}
		snapshot, err := room.Create(owner.Principal.UserID, model.JoinModeOpen, "", "")
		if err != nil {
			t.Fatalf("create room: %v", err)
		}
		ownerEvents, unsubscribe := hub.Subscribe(owner.Principal.UserID)
		defer unsubscribe()

		if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, ""); err != nil {
			t.Fatalf("join room: %v", err)
		}

		assertMemberJoinedEvent(t, ownerEvents, snapshot.RoomCode, guest.Principal.UserID, guest.Principal.DisplayName)
	})

	t.Run("pin", func(t *testing.T) {
		session, _, _ := newSessionTestService(t)
		owner, err := session.Create("曹操", "")
		if err != nil {
			t.Fatalf("create owner: %v", err)
		}
		guest, err := session.Create("孙权", "")
		if err != nil {
			t.Fatalf("create guest: %v", err)
		}
		hub := NewStreamHub()
		room, err := NewRoomSvc(session.db, hub)
		if err != nil {
			t.Fatalf("new room service: %v", err)
		}
		snapshot, err := room.Create(owner.Principal.UserID, model.JoinModePassword, "1234", "1234")
		if err != nil {
			t.Fatalf("create room: %v", err)
		}
		ownerEvents, unsubscribe := hub.Subscribe(owner.Principal.UserID)
		defer unsubscribe()

		if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, "1234"); err != nil {
			t.Fatalf("join room: %v", err)
		}

		assertMemberJoinedEvent(t, ownerEvents, snapshot.RoomCode, guest.Principal.UserID, guest.Principal.DisplayName)
	})

	t.Run("approval", func(t *testing.T) {
		session, _, _ := newSessionTestService(t)
		owner, err := session.Create("周瑜", "")
		if err != nil {
			t.Fatalf("create owner: %v", err)
		}
		guest, err := session.Create("鲁肃", "")
		if err != nil {
			t.Fatalf("create guest: %v", err)
		}
		hub := NewStreamHub()
		room, err := NewRoomSvc(session.db, hub)
		if err != nil {
			t.Fatalf("new room service: %v", err)
		}
		snapshot, err := room.Create(owner.Principal.UserID, model.JoinModeOwnerApproval, "", "")
		if err != nil {
			t.Fatalf("create room: %v", err)
		}
		request, err := room.CreateJoinRequest(guest.Principal.UserID, snapshot.RoomCode)
		if err != nil {
			t.Fatalf("create join request: %v", err)
		}
		ownerEvents, unsubscribe := hub.Subscribe(owner.Principal.UserID)
		defer unsubscribe()

		if err := room.ApproveJoinRequest(owner.Principal.UserID, snapshot.RoomCode, request.RequestID); err != nil {
			t.Fatalf("approve join request: %v", err)
		}

		assertMemberJoinedEvent(t, ownerEvents, snapshot.RoomCode, guest.Principal.UserID, guest.Principal.DisplayName)
	})
}

func assertMemberJoinedEvent(t *testing.T, events <-chan *StreamEvent, roomCode, userID, displayName string) {
	t.Helper()
	select {
	case event := <-events:
		if event.Type != "room.member_joined" {
			t.Fatalf("event type = %q", event.Type)
		}
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("payload type = %T", event.Payload)
		}
		if payload["roomCode"] != roomCode || payload["userId"] != userID || payload["displayName"] != displayName || payload["role"] != model.MemberRoleMember {
			t.Fatalf("event payload = %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("room member did not receive member-joined event")
	}
}

func TestRoomPINAndApprovalModes(t *testing.T) {
	session, _, _ := newSessionTestService(t)
	owner, err := session.Create("曹操", "")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	guest, err := session.Create("孙权", "")
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	room, err := NewRoomSvc(session.db, nil)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	pinRoom, err := room.Create(owner.Principal.UserID, model.JoinModePassword, "1234", "1234")
	if err != nil {
		t.Fatalf("create pin room: %v", err)
	}
	if _, err := room.Join(guest.Principal.UserID, pinRoom.RoomCode, true, "4321"); err == nil {
		t.Fatal("wrong pin unexpectedly succeeded")
	}
	if _, err := room.Join(guest.Principal.UserID, pinRoom.RoomCode, true, "1234"); err != nil {
		t.Fatalf("correct pin join: %v", err)
	}
	approvalOwner, err := session.Create("周瑜", "")
	if err != nil {
		t.Fatalf("create approval owner: %v", err)
	}
	approvalGuest, err := session.Create("鲁肃", "")
	if err != nil {
		t.Fatalf("create approval guest: %v", err)
	}
	approvalRoom, err := room.Create(approvalOwner.Principal.UserID, model.JoinModeOwnerApproval, "", "")
	if err != nil {
		t.Fatalf("create approval room: %v", err)
	}
	request, err := room.CreateJoinRequest(approvalGuest.Principal.UserID, approvalRoom.RoomCode)
	if err != nil {
		t.Fatalf("create join request: %v", err)
	}
	count, requests, err := room.ListJoinRequests(approvalOwner.Principal.UserID, approvalRoom.RoomCode)
	if err != nil || count != 1 || len(requests) != 1 || requests[0].RequestID != request.RequestID {
		t.Fatalf("list join requests = %d, %+v, error %v", count, requests, err)
	}
	if err := room.ApproveJoinRequest(approvalOwner.Principal.UserID, approvalRoom.RoomCode, request.RequestID); err != nil {
		t.Fatalf("approve join request: %v", err)
	}
	joined, err := room.Snapshot(approvalGuest.Principal.UserID, approvalRoom.RoomCode)
	if err != nil {
		t.Fatalf("snapshot approved member: %v", err)
	}
	if joined.Role != model.MemberRoleMember {
		t.Fatalf("approved member role = %q", joined.Role)
	}
}

func TestRoomDissolveAndKick(t *testing.T) {
	session, _, _ := newSessionTestService(t)
	owner, err := session.Create("张辽", "")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	guest, err := session.Create("赵云", "")
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	room, err := NewRoomSvc(session.db, nil)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	snapshot, err := room.Create(owner.Principal.UserID, model.JoinModeOpen, "", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, ""); err != nil {
		t.Fatalf("join guest: %v", err)
	}
	if err := room.Kick(owner.Principal.UserID, snapshot.RoomCode, guest.Principal.UserID); err != nil {
		t.Fatalf("kick guest: %v", err)
	}
	if _, err := room.Join(guest.Principal.UserID, snapshot.RoomCode, true, ""); err == nil {
		t.Fatal("kicked guest unexpectedly rejoined")
	}
	if err := room.Dissolve(owner.Principal.UserID, snapshot.RoomCode); err != nil {
		t.Fatalf("dissolve room: %v", err)
	}
	destroying, err := room.Snapshot(owner.Principal.UserID, snapshot.RoomCode)
	if err != nil {
		t.Fatalf("destroying snapshot: %v", err)
	}
	if destroying.Status != model.RoomStatusDestroying || destroying.DestroyAt == nil {
		t.Fatalf("unexpected destroying snapshot: %+v", destroying)
	}
}
