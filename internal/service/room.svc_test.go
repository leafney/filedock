package service

import (
	"testing"

	"github.com/leafney/filedock/internal/model"
)

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
	if joined.Role != model.MemberRoleMember || len(joined.Members) != 2 {
		t.Fatalf("unexpected joined snapshot: %+v", joined)
	}
	if err := room.Leave(owner.Principal.UserID, snapshot.RoomCode); err == nil {
		t.Fatal("owner leave unexpectedly succeeded")
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
