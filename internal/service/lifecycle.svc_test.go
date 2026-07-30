package service

import (
	"errors"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/ulidx"
)

func TestLifecycleExpiresAndDestroysRoom(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	hub := NewStreamHub()
	lifecycle, err := NewLifecycleSvc(db, hub)
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	roomID := "01ROOMLIFECYCLE01"
	userID := "01USERLIFECYCLE01"
	base := time.Unix(1_000_000, 0)
	if err := db.Create(&model.Room{ID: roomID, Code: "0001", Title: "房间 0001", OwnerUserID: userID, JoinMode: model.JoinModeOpen, Status: model.RoomStatusActive, CreatedAt: base.Add(-time.Hour).Unix(), ExpiresAt: base.Add(-time.Second).Unix()}).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := db.Create(&model.RoomMember{ID: "01MEMBERLIFECYCLE1", RoomID: roomID, UserID: userID, Role: model.MemberRoleOwner, DisplayName: "房主", Status: model.MemberStatusActive, JoinedAt: base.Unix(), LastSeenAt: base.Unix()}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	lifecycle.now = func() time.Time { return base }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("expire tick: %v", err)
	}
	var room model.Room
	if err := db.First(&room, "id = ?", roomID).Error; err != nil {
		t.Fatalf("load destroying room: %v", err)
	}
	if room.Status != model.RoomStatusDestroying || room.DestroyAt == nil || *room.DestroyAt != base.Add(RoomDestroyDelay).Unix() {
		t.Fatalf("unexpected destroying room: %+v", room)
	}

	lifecycle.now = func() time.Time { return base.Add(RoomDestroyDelay + time.Second) }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("destroy tick: %v", err)
	}
	if err := db.First(&room, "id = ?", roomID).Error; err != nil {
		t.Fatalf("load destroyed room: %v", err)
	}
	if room.Status != model.RoomStatusDestroyed || room.DestroyedAt == nil || room.CodeReusableAt == nil {
		t.Fatalf("unexpected destroyed room: %+v", room)
	}
	var member model.RoomMember
	if err := db.First(&member, "room_id = ?", roomID).Error; err != nil {
		t.Fatalf("load destroyed member: %v", err)
	}
	if member.Status != model.MemberStatusDestroyed {
		t.Fatalf("member status = %q, want destroyed", member.Status)
	}
	var job model.CleanupJob
	if err := db.First(&job, "room_id = ?", roomID).Error; err != nil {
		t.Fatalf("load cleanup job: %v", err)
	}
	if job.Status != model.CleanupPending || job.ScheduledAt < base.Add(CleanupMinDelay).Unix() || job.ScheduledAt > base.Add(CleanupMaxDelay).Unix() {
		t.Fatalf("unexpected cleanup job: %+v", job)
	}
}

func TestLifecycleCleanupRetriesAtMostOnce(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	base := time.Unix(2_000_000, 0)
	jobID, err := ulidx.New()
	if err != nil {
		t.Fatalf("generate job id: %v", err)
	}
	if err := db.Create(&model.CleanupJob{ID: jobID, RoomID: "01ROOMRETRY000001", Status: model.CleanupPending, Phase: "scheduled", ScheduledAt: base.Unix()}).Error; err != nil {
		t.Fatalf("create cleanup job: %v", err)
	}
	lifecycle.now = func() time.Time { return base }
	lifecycle.cleanup = func(string, string, time.Time) error { return errors.New("simulated cleanup failure") }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("first cleanup tick: %v", err)
	}
	var job model.CleanupJob
	if err := db.First(&job, "id = ?", jobID).Error; err != nil {
		t.Fatalf("load first cleanup job: %v", err)
	}
	if job.Status != model.CleanupRetryWaiting || job.AttemptCount != 1 {
		t.Fatalf("first failure job = %+v", job)
	}
	if err := db.Model(&model.CleanupJob{}).Where("id = ?", jobID).Update("scheduled_at", base.Unix()).Error; err != nil {
		t.Fatalf("schedule retry: %v", err)
	}
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("second cleanup tick: %v", err)
	}
	if err := db.First(&job, "id = ?", jobID).Error; err != nil {
		t.Fatalf("load second cleanup job: %v", err)
	}
	if job.Status != model.CleanupFailed || job.AttemptCount != 2 {
		t.Fatalf("second failure job = %+v", job)
	}
	if err := db.Model(&model.CleanupJob{}).Where("id = ?", jobID).Update("scheduled_at", base.Add(-time.Hour).Unix()).Error; err != nil {
		t.Fatalf("make failed job due: %v", err)
	}
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("failed job tick: %v", err)
	}
	if err := db.First(&job, "id = ?", jobID).Error; err != nil {
		t.Fatalf("load final cleanup job: %v", err)
	}
	if job.AttemptCount != 2 {
		t.Fatalf("failed job was retried, attempts = %d", job.AttemptCount)
	}
}

func TestLifecycleReleasesRoomAfterCooldownAndCleanup(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	base := time.Unix(3_000_000, 0)
	roomID := "01ROOMRELEASE0001"
	reusableAt := base.Add(RoomCodeCooldown)
	destroyedAt := base.Unix()
	codeReusableAt := reusableAt.Unix()
	if err := db.Create(&model.Room{ID: roomID, Code: "0002", Title: "房间 0002", OwnerUserID: "01USERRELEASE0001", JoinMode: model.JoinModeOpen, Status: model.RoomStatusDestroyed, CreatedAt: base.Add(-time.Hour).Unix(), ExpiresAt: base.Unix(), DestroyedAt: &destroyedAt, CodeReusableAt: &codeReusableAt}).Error; err != nil {
		t.Fatalf("create destroyed room: %v", err)
	}
	jobID, err := ulidx.New()
	if err != nil {
		t.Fatalf("generate job id: %v", err)
	}
	finishedAt := base.Unix()
	if err := db.Create(&model.CleanupJob{ID: jobID, RoomID: roomID, Status: model.CleanupSucceeded, Phase: "completed", ScheduledAt: base.Unix(), FinishedAt: &finishedAt}).Error; err != nil {
		t.Fatalf("create cleanup job: %v", err)
	}
	lifecycle.now = func() time.Time { return reusableAt.Add(time.Second) }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("release tick: %v", err)
	}
	if err := db.First(&model.Room{}, "id = ?", roomID).Error; err == nil {
		t.Fatal("room still exists after cooldown and cleanup")
	}
	if err := db.First(&model.CleanupJob{}, "id = ?", jobID).Error; err == nil {
		t.Fatal("cleanup job still exists after room release")
	}
}

func TestLifecycleExpiresJoinRequests(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	base := time.Unix(4_000_000, 0)
	if err := db.Create(&model.JoinRequest{ID: "01REQUESTLIFECYCLE1", RoomID: "01ROOMREQUEST0001", UserID: "01USERREQUEST0001", DisplayName: "申请人", Status: model.JoinRequestPending, CreatedAt: base.Add(-time.Minute).Unix(), ExpiresAt: base.Add(-time.Second).Unix()}).Error; err != nil {
		t.Fatalf("create join request: %v", err)
	}
	lifecycle.now = func() time.Time { return base }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("expire request tick: %v", err)
	}
	var request model.JoinRequest
	if err := db.First(&request, "id = ?", "01REQUESTLIFECYCLE1").Error; err != nil {
		t.Fatalf("load expired request: %v", err)
	}
	if request.Status != model.JoinRequestExpired || request.NextAllowedAt == nil || *request.NextAllowedAt != base.Add(ApprovalCancelWait).Unix() {
		t.Fatalf("unexpected expired request: %+v", request)
	}
}
