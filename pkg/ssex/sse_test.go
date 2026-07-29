package ssex

import "testing"

func TestHubSubscribeFilterAndUnsubscribe(t *testing.T) {
	hub := NewHub()
	ch, unsubscribe := hub.Subscribe(func(event *Event) bool {
		return event != nil && event.EventType == "keep"
	})

	hub.Publish(NewEvent("drop", map[string]string{"value": "x"}))
	select {
	case <-ch:
		t.Fatal("unexpected event received")
	default:
	}

	hub.Publish(NewEvent("keep", map[string]string{"value": "ok"}))
	event := <-ch
	if event == nil || event.EventType != "keep" {
		t.Fatalf("unexpected event: %#v", event)
	}

	unsubscribe()
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after unsubscribe")
	}
}
