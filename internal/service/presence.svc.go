package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/leafney/filedock/internal/model"
	"gorm.io/gorm"
)

const (
	PresenceOnline       = "online"
	PresenceAway         = "away"
	PresenceOffline      = "offline"
	PresenceOfflineDelay = 60 * time.Second
)

type presenceUser struct {
	connections int
	status      string
	generation  uint64
}

type PresenceSvc struct {
	db    *gorm.DB
	hub   *StreamHub
	now   func() time.Time
	mu    sync.Mutex
	users map[string]*presenceUser
}

func NewPresenceSvc(db *gorm.DB, hub *StreamHub) (*PresenceSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("presence database is required")
	}
	if hub == nil {
		return nil, fmt.Errorf("presence stream hub is required")
	}
	return &PresenceSvc{db: db, hub: hub, now: time.Now, users: make(map[string]*presenceUser)}, nil
}

func (p *PresenceSvc) Connect(userID string) func() {
	if p == nil || userID == "" {
		return func() {}
	}
	p.mu.Lock()
	user := p.users[userID]
	if user == nil {
		user = &presenceUser{status: PresenceOffline}
		p.users[userID] = user
	}
	user.connections++
	changed := user.status != PresenceOnline
	user.status = PresenceOnline
	user.generation++
	generation := user.generation
	p.mu.Unlock()
	if changed {
		p.broadcast(userID, PresenceOnline)
	}
	var once sync.Once
	return func() {
		once.Do(func() { p.disconnect(userID, generation) })
	}
}

func (p *PresenceSvc) disconnect(userID string, generation uint64) {
	p.mu.Lock()
	user := p.users[userID]
	if user == nil || user.connections <= 0 {
		p.mu.Unlock()
		return
	}
	user.connections--
	if user.connections > 0 {
		p.mu.Unlock()
		return
	}
	user.status = PresenceAway
	user.generation++
	currentGeneration := user.generation
	p.mu.Unlock()
	p.broadcast(userID, PresenceAway)
	time.AfterFunc(PresenceOfflineDelay, func() {
		p.mu.Lock()
		current := p.users[userID]
		if current == nil || current.connections > 0 || current.generation != currentGeneration || generation == 0 {
			p.mu.Unlock()
			return
		}
		current.status = PresenceOffline
		p.mu.Unlock()
		p.broadcast(userID, PresenceOffline)
	})
}

func (p *PresenceSvc) Status(userID string) string {
	if p == nil || userID == "" {
		return PresenceOffline
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if user := p.users[userID]; user != nil && user.status != "" {
		return user.status
	}
	return PresenceOffline
}

func (p *PresenceSvc) broadcast(userID, status string) {
	var members []model.RoomMember
	if err := p.db.Where("user_id = ? AND status = ?", userID, model.MemberStatusActive).Find(&members).Error; err != nil {
		return
	}
	for _, member := range members {
		var roomMembers []model.RoomMember
		if err := p.db.Where("room_id = ? AND status = ?", member.RoomID, model.MemberStatusActive).Find(&roomMembers).Error; err != nil {
			continue
		}
		userIDs := make([]string, 0, len(roomMembers))
		for _, roomMember := range roomMembers {
			userIDs = append(userIDs, roomMember.UserID)
		}
		var room model.Room
		if err := p.db.Select("code").Where("id = ?", member.RoomID).First(&room).Error; err != nil {
			continue
		}
		p.hub.PublishUsers(userIDs, "room.member_presence_changed", map[string]interface{}{"roomCode": room.Code, "userId": userID, "status": status})
	}
}
