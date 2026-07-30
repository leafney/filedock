package api

import (
	"bufio"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
)

type StreamAPI struct {
	hub      *service.StreamHub
	presence *service.PresenceSvc
}

func NewStreamAPI(hub *service.StreamHub, presence *service.PresenceSvc) (*StreamAPI, error) {
	if hub == nil {
		return nil, fmt.Errorf("stream hub is required")
	}
	if presence == nil {
		return nil, fmt.Errorf("presence service is required")
	}
	return &StreamAPI{hub: hub, presence: presence}, nil
}

func (a *StreamAPI) Handle(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	events, unsubscribe := a.hub.Subscribe(principal.UserID)
	disconnect := a.presence.Connect(principal.UserID)
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Context().SetBodyStreamWriter(func(writer *bufio.Writer) {
		defer unsubscribe()
		defer disconnect()
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case event, open := <-events:
				if !open {
					return
				}
				if err := writeSSEEvent(writer, event); err != nil {
					return
				}
			case <-heartbeat.C:
				if _, err := writer.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				if err := writer.Flush(); err != nil {
					return
				}
			}
		}
	})
	return nil
}

func writeSSEEvent(writer *bufio.Writer, event *service.StreamEvent) error {
	data, err := service.MarshalStreamEvent(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data); err != nil {
		return err
	}
	return writer.Flush()
}
