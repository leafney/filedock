package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leafney/filedock/pkg/ulidx"
)

type StreamEvent struct {
	Version    int         `json:"version"`
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	OccurredAt string      `json:"occurredAt"`
	Payload    interface{} `json:"payload,omitempty"`
}

type streamSubscriber struct {
	userID string
	ch     chan *StreamEvent
}

type StreamHub struct {
	nextID uint64
	mu     sync.RWMutex
	subs   map[uint64]*streamSubscriber
}

func NewStreamHub() *StreamHub {
	return &StreamHub{subs: make(map[uint64]*streamSubscriber)}
}

func (h *StreamHub) Subscribe(userID string) (<-chan *StreamEvent, func()) {
	if h == nil {
		closed := make(chan *StreamEvent)
		close(closed)
		return closed, func() {}
	}
	id := atomic.AddUint64(&h.nextID, 1)
	subscriber := &streamSubscriber{userID: userID, ch: make(chan *StreamEvent, 32)}
	h.mu.Lock()
	h.subs[id] = subscriber
	h.mu.Unlock()
	return subscriber.ch, func() { h.remove(id) }
}

func (h *StreamHub) remove(id uint64) {
	if h == nil {
		return
	}
	h.mu.Lock()
	if subscriber, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(subscriber.ch)
	}
	h.mu.Unlock()
}

func (h *StreamHub) PublishUser(userID, eventType string, payload interface{}) {
	h.PublishUsers([]string{userID}, eventType, payload)
}

func (h *StreamHub) PublishUsers(userIDs []string, eventType string, payload interface{}) {
	if h == nil || eventType == "" {
		return
	}
	eventID, err := ulidx.New()
	if err != nil {
		eventID = fmt.Sprintf("event-%d", time.Now().UnixNano())
	}
	event := &StreamEvent{Version: 1, ID: eventID, Type: eventType, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: payload}
	allowed := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID != "" {
			allowed[userID] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return
	}
	var slow []uint64
	h.mu.RLock()
	for id, subscriber := range h.subs {
		if _, ok := allowed[subscriber.userID]; !ok {
			continue
		}
		select {
		case subscriber.ch <- event:
		default:
			slow = append(slow, id)
		}
	}
	h.mu.RUnlock()
	for _, id := range slow {
		h.remove(id)
	}
}

func (h *StreamHub) CountUserConnections(userID string) int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, subscriber := range h.subs {
		if subscriber.userID == userID {
			count++
		}
	}
	return count
}

func MarshalStreamEvent(event *StreamEvent) ([]byte, error) { return json.Marshal(event) }
