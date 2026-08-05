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

func TestLifecycleCleanupDeletesChatRelationsIdempotently(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	roomID := "01ROOMCHATCLEANUP01"
	conversationID := "01CHATCLEANUPCONV01"
	messageID := "01CHATCLEANUPMESSAGE1"
	jobID := "01JOBCHATCLEANUP001"
	if err := db.Create(&model.Room{ID: roomID, Code: "0098", Title: "聊天清理", OwnerUserID: "01USERCHATCLEANUP1", JoinMode: model.JoinModeOpen, Status: model.RoomStatusDestroyed, CreatedAt: 1, ExpiresAt: 2}).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := db.Create(&model.ChatConversation{ID: conversationID, RoomID: roomID, UserAID: "01USERCHATCLEANUP1", UserBID: "01USERCHATCLEANUP2", LastSequence: 1, CreatedAt: 1, UpdatedAt: 1}).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&model.ChatMessage{ID: messageID, RoomID: roomID, ConversationID: conversationID, SenderUserID: "01USERCHATCLEANUP1", RecipientUserID: "01USERCHATCLEANUP2", SenderDisplayName: "甲一", ClientMessageID: "cleanup", ContentText: "hello", Sequence: 1, CreatedAt: 1}).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := db.Create(&model.ChatReadState{ConversationID: conversationID, UserID: "01USERCHATCLEANUP2", LastReadSequence: 1, LastReadAt: 1, UpdatedAt: 1}).Error; err != nil {
		t.Fatalf("create read state: %v", err)
	}
	if err := db.Create(&model.ChatMessageDeletion{MessageID: messageID, UserID: "01USERCHATCLEANUP1", DeletedAt: 1}).Error; err != nil {
		t.Fatalf("create deletion: %v", err)
	}
	if err := db.Create(&model.NotificationRecord{ID: "01NOTIFYCHATCLEANUP01", UserID: "01USERCHATCLEANUP1", Type: NotificationTypeFileDeclined, RoomID: roomID, FileID: "01FILECHATCLEANUP001", CounterpartUserID: "01USERCHATCLEANUP2", FileRecipientID: "01RECIPIENTCLEANUP01", DeliveryVersion: 1, SourceEventID: "01EVENTCHATCLEANUP01", OccurredAtMS: 1000}).Error; err != nil {
		t.Fatalf("create notification record: %v", err)
	}
	if err := db.Create(&model.CleanupJob{ID: jobID, RoomID: roomID, Status: model.CleanupRunning, Phase: "running", ScheduledAt: 1}).Error; err != nil {
		t.Fatalf("create cleanup job: %v", err)
	}
	if err := lifecycle.runCleanup(roomID, jobID, time.Unix(2, 0)); err != nil {
		t.Fatalf("run chat cleanup: %v", err)
	}
	if err := lifecycle.runCleanup(roomID, jobID, time.Unix(3, 0)); err != nil {
		t.Fatalf("repeat chat cleanup: %v", err)
	}
	for _, table := range []string{"chat_message_deletions", "chat_read_states", "chat_messages", "chat_conversations", "notification_records"} {
		var count int64
		if err := db.Table(table).Where("1 = 1").Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", table, count)
		}
	}
	var job model.CleanupJob
	if err := db.First(&job, "id = ?", jobID).Error; err != nil {
		t.Fatalf("load cleanup job: %v", err)
	}
	if job.TotalNotifications != 1 || job.CleanedNotifications != 1 {
		t.Fatalf("notification cleanup stats = %+v", job)
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

func TestLifecycleRecoveryDoesNotExceedCleanupAttempts(t *testing.T) {
	_, db, _ := newSessionTestService(t)
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	base := time.Unix(2_500_000, 0)
	firstID, err := ulidx.New()
	if err != nil {
		t.Fatalf("generate first job id: %v", err)
	}
	secondID, err := ulidx.New()
	if err != nil {
		t.Fatalf("generate second job id: %v", err)
	}
	if err := db.Create(&model.CleanupJob{ID: firstID, RoomID: "01ROOMRECOVER0001", Status: model.CleanupRunning, Phase: "running", AttemptCount: 1, ScheduledAt: base.Unix()}).Error; err != nil {
		t.Fatalf("create first running job: %v", err)
	}
	if err := db.Create(&model.CleanupJob{ID: secondID, RoomID: "01ROOMRECOVER0002", Status: model.CleanupRunning, Phase: "running", AttemptCount: 2, ScheduledAt: base.Unix()}).Error; err != nil {
		t.Fatalf("create second running job: %v", err)
	}
	lifecycle.now = func() time.Time { return base }
	if err := lifecycle.recoverCleanupJobs(); err != nil {
		t.Fatalf("recover cleanup jobs: %v", err)
	}
	var first, second model.CleanupJob
	if err := db.First(&first, "id = ?", firstID).Error; err != nil {
		t.Fatalf("load first job: %v", err)
	}
	if err := db.First(&second, "id = ?", secondID).Error; err != nil {
		t.Fatalf("load second job: %v", err)
	}
	if first.Status != model.CleanupRetryWaiting || first.AttemptCount != 1 {
		t.Fatalf("first recovered job = %+v", first)
	}
	if second.Status != model.CleanupFailed || second.AttemptCount != 2 {
		t.Fatalf("second recovered job = %+v", second)
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

func TestLifecycleExpiresSessionsAndReleasesNickname(t *testing.T) {
	session, db, _ := newSessionTestService(t)
	created, err := session.Create("周瑜", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	base := time.Unix(5_000_000, 0)
	if err := db.Model(&model.Session{}).Where("id = ?", created.Principal.SessionID).Updates(map[string]interface{}{"expires_at": base.Add(-time.Second).Unix(), "revoked_at": nil}).Error; err != nil {
		t.Fatalf("expire session: %v", err)
	}
	lifecycle, err := NewLifecycleSvc(db, NewStreamHub())
	if err != nil {
		t.Fatalf("NewLifecycleSvc() error = %v", err)
	}
	lifecycle.now = func() time.Time { return base }
	if err := lifecycle.Tick(); err != nil {
		t.Fatalf("expire session tick: %v", err)
	}
	var storedSession model.Session
	if err := db.First(&storedSession, "id = ?", created.Principal.SessionID).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if storedSession.RevokedAt == nil {
		t.Fatal("expired session was not revoked")
	}
	var user model.User
	if err := db.First(&user, "id = ?", created.Principal.UserID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.Status != model.UserStatusExpired || user.DisplayNameKey != "" {
		t.Fatalf("user after session expiry = %+v", user)
	}
}
