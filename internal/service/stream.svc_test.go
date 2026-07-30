package service

import (
	"testing"
	"time"
)

func TestStreamHubPublishesOnlyToUserConnections(t *testing.T) {
	hub := NewStreamHub()
	first, unsubscribeFirst := hub.Subscribe("user-a")
	second, unsubscribeSecond := hub.Subscribe("user-a")
	other, unsubscribeOther := hub.Subscribe("user-b")
	defer unsubscribeFirst()
	defer unsubscribeSecond()
	defer unsubscribeOther()

	hub.PublishUser("user-a", "room.member_joined", map[string]string{"userId": "user-c"})
	for name, channel := range map[string]<-chan *StreamEvent{"first": first, "second": second} {
		select {
		case event := <-channel:
			if event.Type != "room.member_joined" {
				t.Fatalf("%s event type = %q", name, event.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive event", name)
		}
	}
	select {
	case event := <-other:
		t.Fatalf("other user received event: %+v", event)
	default:
	}
}

func TestStreamHubClosesSlowSubscriber(t *testing.T) {
	hub := NewStreamHub()
	events, _ := hub.Subscribe("slow")
	for i := 0; i < 40; i++ {
		hub.PublishUser("slow", "system.test", i)
	}
	for range events {
	}
	if hub.CountUserConnections("slow") != 0 {
		t.Fatal("slow subscriber was not removed")
	}
}

func TestStreamHubClosesAllUserConnections(t *testing.T) {
	hub := NewStreamHub()
	first, _ := hub.Subscribe("user-a")
	second, _ := hub.Subscribe("user-a")
	_, unsubscribeOther := hub.Subscribe("user-b")
	defer unsubscribeOther()

	hub.CloseUser("user-a")
	for name, channel := range map[string]<-chan *StreamEvent{"first": first, "second": second} {
		select {
		case _, open := <-channel:
			if open {
				t.Fatalf("%s channel is still open", name)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s channel did not close", name)
		}
	}
	if hub.CountUserConnections("user-a") != 0 {
		t.Fatal("user-a connections were not removed")
	}
	if hub.CountUserConnections("user-b") != 1 {
		t.Fatal("user-b connection was unexpectedly removed")
	}
}
