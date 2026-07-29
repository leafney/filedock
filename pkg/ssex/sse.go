package ssex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	EventType       string      `json:"event_type"`
	OccurredAt      string      `json:"occurred_at"`
	Payload         interface{} `json:"payload"`
	TargetRoles     []string    `json:"-"`
	TargetUserTypes []string    `json:"-"`
}

type subscriber struct {
	ch     chan *Event
	filter func(*Event) bool
}

type Hub struct {
	nextID uint64
	mu     sync.RWMutex
	subs   map[uint64]*subscriber
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[uint64]*subscriber),
	}
}

func NewEvent(eventType string, payload interface{}) *Event {
	return &Event{
		EventType:  eventType,
		OccurredAt: time.Now().Format(time.RFC3339),
		Payload:    payload,
	}
}

func (h *Hub) Subscribe(filter func(*Event) bool) (<-chan *Event, func()) {
	id := atomic.AddUint64(&h.nextID, 1)
	sub := &subscriber{
		ch:     make(chan *Event, 16),
		filter: filter,
	}
	h.mu.Lock()
	h.subs[id] = sub
	h.mu.Unlock()

	return sub.ch, func() {
		h.mu.Lock()
		if current, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(current.ch)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(event *Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
		}
	}
}

func WriteEvent(w *bufio.Writer, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return w.Flush()
}
