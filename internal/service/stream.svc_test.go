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
