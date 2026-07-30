package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	CleanupMinDelay = 10 * time.Second
	CleanupMaxDelay = 5 * time.Minute
)

type LifecycleSvc struct {
	db  *gorm.DB
	hub *StreamHub
	now func() time.Time

	mu      sync.Mutex
	started bool
	stop    chan struct{}
	done    chan struct{}
	cleanup func(roomID, jobID string, now time.Time) error
}

func NewLifecycleSvc(db *gorm.DB, hub *StreamHub) (*LifecycleSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("lifecycle database is required")
	}
	if hub == nil {
		return nil, fmt.Errorf("lifecycle stream hub is required")
	}
	service := &LifecycleSvc{db: db, hub: hub, now: time.Now}
	service.cleanup = service.runCleanup
	return service, nil
}

func (s *LifecycleSvc) Start(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("lifecycle service is nil")
	}
	if ctx == nil {
		return fmt.Errorf("lifecycle context is required")
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	stop := s.stop
	done := s.done
	s.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				_ = s.Tick()
			}
		}
	}()
	if err := s.recoverCleanupJobs(); err != nil {
		_ = s.Stop()
		return err
	}
	if err := s.Tick(); err != nil {
		_ = s.Stop()
		return err
	}
	return nil
}

func (s *LifecycleSvc) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	close(s.stop)
	done := s.done
	s.started = false
	s.mu.Unlock()
	<-done
	return nil
}

func (s *LifecycleSvc) Tick() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("lifecycle service is nil")
	}
	now := s.now()
	if err := s.expireSessions(now); err != nil {
		return err
	}
	if err := s.expireJoinRequests(now); err != nil {
		return err
	}
	if err := s.startExpiredRooms(now); err != nil {
		return err
	}
	if err := s.finishDestroyingRooms(now); err != nil {
		return err
	}
	if err := s.processCleanupJobs(now); err != nil {
		return err
	}
	return s.releaseRooms(now)
}

func (s *LifecycleSvc) expireSessions(now time.Time) error {
	var sessions []model.Session
	if err := s.db.Where("revoked_at IS NULL AND expires_at <= ?", now.Unix()).Find(&sessions).Error; err != nil {
		return err
	}
	for _, session := range sessions {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.Session{}).Where("id = ? AND revoked_at IS NULL", session.ID).Updates(map[string]interface{}{"revoked_at": now.Unix(), "updated_at": now.Unix()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			var active int64
			if err := tx.Model(&model.Session{}).Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", session.UserID, now.Unix()).Count(&active).Error; err != nil {
				return err
			}
			if active > 0 {
				return nil
			}
			return tx.Model(&model.User{}).Where("id = ? AND status = ?", session.UserID, model.UserStatusActive).Updates(map[string]interface{}{"status": model.UserStatusExpired, "display_name_key": "", "identity_expires_at": now.Unix(), "updated_at": now.Unix()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

// recoverCleanupJobs makes tasks interrupted during a previous process run
// executable again without giving them an extra attempt.
func (s *LifecycleSvc) recoverCleanupJobs() error {
	var jobs []model.CleanupJob
	if err := s.db.Where("status = ?", model.CleanupRunning).Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		updates := map[string]interface{}{"status": model.CleanupRetryWaiting, "phase": "recovered", "scheduled_at": s.now().Unix()}
		if job.AttemptCount >= 2 {
			updates = map[string]interface{}{"status": model.CleanupFailed, "phase": "failed", "finished_at": s.now().Unix(), "error_summary": "cleanup interrupted after retry"}
		}
		if err := s.db.Model(&model.CleanupJob{}).Where("id = ? AND status = ?", job.ID, model.CleanupRunning).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *LifecycleSvc) expireJoinRequests(now time.Time) error {
	var requests []model.JoinRequest
	if err := s.db.Where("status = ? AND expires_at <= ?", model.JoinRequestPending, now.Unix()).Find(&requests).Error; err != nil {
		return err
	}
	for _, request := range requests {
		next := now.Add(ApprovalCancelWait).Unix()
		if err := s.db.Model(&model.JoinRequest{}).Where("id = ? AND status = ?", request.ID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestExpired, "processed_at": now.Unix(), "next_allowed_at": next}).Error; err != nil {
			return err
		}
		var room model.Room
		if err := s.db.Select("code, owner_user_id").Where("id = ?", request.RoomID).First(&room).Error; err == nil {
			s.hub.PublishUsers([]string{request.UserID, room.OwnerUserID}, "room.join_request_changed", map[string]interface{}{"roomCode": room.Code, "requestId": request.ID, "status": model.JoinRequestExpired})
		}
	}
	return nil
}

func (s *LifecycleSvc) startExpiredRooms(now time.Time) error {
	var rooms []model.Room
	if err := s.db.Where("status = ? AND expires_at <= ?", model.RoomStatusActive, now.Unix()).Find(&rooms).Error; err != nil {
		return err
	}
	for _, room := range rooms {
		result := s.db.Model(&model.Room{}).Where("id = ? AND status = ?", room.ID, model.RoomStatusActive).Updates(map[string]interface{}{"status": model.RoomStatusDestroying, "destroying_at": now.Unix(), "destroy_at": now.Add(RoomDestroyDelay).Unix()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			s.publishRoom(room.ID, "room.destroying", map[string]interface{}{"roomCode": room.Code, "reason": "expired", "destroyAt": now.Add(RoomDestroyDelay).Unix()})
		}
	}
	return nil
}

func (s *LifecycleSvc) finishDestroyingRooms(now time.Time) error {
	var rooms []model.Room
	if err := s.db.Where("status = ? AND destroy_at IS NOT NULL AND destroy_at <= ?", model.RoomStatusDestroying, now.Unix()).Find(&rooms).Error; err != nil {
		return err
	}
	for _, room := range rooms {
		var userIDs []string
		var members []model.RoomMember
		if err := s.db.Where("room_id = ? AND status = ?", room.ID, model.MemberStatusActive).Find(&members).Error; err != nil {
			return err
		}
		for _, member := range members {
			userIDs = append(userIDs, member.UserID)
		}
		codeReusableAt := now.Add(RoomCodeCooldown).Unix()
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.Room{}).Where("id = ? AND status = ?", room.ID, model.RoomStatusDestroying).Updates(map[string]interface{}{"status": model.RoomStatusDestroyed, "destroyed_at": now.Unix(), "code_reusable_at": codeReusableAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			if err := tx.Model(&model.RoomMember{}).Where("room_id = ? AND status = ?", room.ID, model.MemberStatusActive).Updates(map[string]interface{}{"status": model.MemberStatusDestroyed}).Error; err != nil {
				return err
			}
			return tx.Model(&model.JoinRequest{}).Where("room_id = ? AND status = ?", room.ID, model.JoinRequestPending).Updates(map[string]interface{}{"status": model.JoinRequestExpired, "processed_at": now.Unix()}).Error
		}); err != nil {
			return err
		}
		jobID, err := ulidx.New()
		if err != nil {
			return err
		}
		delay, err := randomCleanupDelay()
		if err != nil {
			return err
		}
		job := model.CleanupJob{ID: jobID, RoomID: room.ID, Status: model.CleanupPending, Phase: "scheduled", ScheduledAt: now.Add(delay).Unix()}
		if err := s.db.Where("room_id = ?", room.ID).FirstOrCreate(&job).Error; err != nil {
			return err
		}
		s.hub.PublishUsers(userIDs, "room.destroyed", map[string]interface{}{"roomCode": room.Code, "reason": "destroyed"})
	}
	return nil
}

func randomCleanupDelay() (time.Duration, error) {
	rangeSize := int64((CleanupMaxDelay - CleanupMinDelay) / time.Second)
	value, err := rand.Int(rand.Reader, big.NewInt(rangeSize+1))
	if err != nil {
		return 0, fmt.Errorf("randomize cleanup delay: %w", err)
	}
	return CleanupMinDelay + time.Duration(value.Int64())*time.Second, nil
}

func (s *LifecycleSvc) processCleanupJobs(now time.Time) error {
	var jobs []model.CleanupJob
	if err := s.db.Where("status IN ? AND scheduled_at <= ?", []string{model.CleanupPending, model.CleanupRetryWaiting}, now.Unix()).Order("scheduled_at ASC").Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		result := s.db.Model(&model.CleanupJob{}).Where("id = ? AND status IN ?", job.ID, []string{model.CleanupPending, model.CleanupRetryWaiting}).Updates(map[string]interface{}{"status": model.CleanupRunning, "phase": "running", "attempt_count": gorm.Expr("attempt_count + 1"), "started_at": now.Unix(), "last_error": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		cleanup := s.cleanup
		if cleanup == nil {
			cleanup = s.runCleanup
		}
		if err := cleanup(job.RoomID, job.ID, now); err != nil {
			var current model.CleanupJob
			if loadErr := s.db.Where("id = ?", job.ID).First(&current).Error; loadErr != nil {
				return loadErr
			}
			if current.AttemptCount < 2 {
				delay, delayErr := randomCleanupDelay()
				if delayErr != nil {
					return delayErr
				}
				if updateErr := s.db.Model(&model.CleanupJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{"status": model.CleanupRetryWaiting, "phase": "retry_waiting", "scheduled_at": now.Add(delay).Unix(), "last_error": err.Error(), "error_summary": "cleanup attempt failed"}).Error; updateErr != nil {
					return updateErr
				}
			} else if updateErr := s.db.Model(&model.CleanupJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{"status": model.CleanupFailed, "phase": "failed", "finished_at": now.Unix(), "last_error": err.Error(), "error_summary": "cleanup failed after retry"}).Error; updateErr != nil {
				return updateErr
			}
			continue
		}
		if err := s.db.Model(&model.CleanupJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{"status": model.CleanupSucceeded, "phase": "completed", "finished_at": now.Unix(), "last_error": "", "error_summary": ""}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *LifecycleSvc) runCleanup(roomID, jobID string, now time.Time) error {
	var members int64
	if err := s.db.Model(&model.RoomMember{}).Where("room_id = ?", roomID).Count(&members).Error; err != nil {
		return err
	}
	var requests int64
	if err := s.db.Model(&model.JoinRequest{}).Where("room_id = ?", roomID).Count(&requests).Error; err != nil {
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", roomID).Delete(&model.RoomMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&model.JoinRequest{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.CleanupJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{"phase": "relations_deleted", "total_members": members, "cleaned_members": members, "total_join_requests": requests, "cleaned_join_requests": requests, "last_error": "", "started_at": now.Unix()}).Error
	}); err != nil {
		return err
	}
	return nil
}

func (s *LifecycleSvc) releaseRooms(now time.Time) error {
	var rooms []model.Room
	if err := s.db.Where("status = ? AND code_reusable_at IS NOT NULL AND code_reusable_at <= ?", model.RoomStatusDestroyed, now.Unix()).Find(&rooms).Error; err != nil {
		return err
	}
	for _, room := range rooms {
		var job model.CleanupJob
		if err := s.db.Where("room_id = ?", room.ID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if job.Status != model.CleanupSucceeded {
			continue
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&model.CleanupJob{}, "id = ?", job.ID).Error; err != nil {
				return err
			}
			return tx.Delete(&model.Room{}, "id = ? AND status = ?", room.ID, model.RoomStatusDestroyed).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LifecycleSvc) publishRoom(roomID, eventType string, payload interface{}) {
	var members []model.RoomMember
	if err := s.db.Where("room_id = ?", roomID).Find(&members).Error; err != nil {
		return
	}
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	s.hub.PublishUsers(userIDs, eventType, payload)
}
