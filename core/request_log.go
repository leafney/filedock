package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/pkg/zlogx"
	"go.uber.org/zap"
)

const requestIDLocalKey = "filedock_request_id"

type RequestLogger struct {
	log requestLogSink
}

type requestLogSink interface {
	Info(string, ...zap.Field)
	Warn(string, ...zap.Field)
	Error(string, ...zap.Field)
}

func NewRequestLogger(log *zlogx.ZLogSvc) (*RequestLogger, error) {
	if log == nil {
		return nil, fmt.Errorf("request logger dependency is required")
	}
	return &RequestLogger{log: log}, nil
}

func (m *RequestLogger) Handle(c *fiber.Ctx) error {
	startedAt := time.Now()
	requestID, err := newRequestID()
	if err != nil {
		m.log.Error("generate request id", zap.Error(err))
		return fiber.ErrInternalServerError
	}
	c.Locals(requestIDLocalKey, requestID)
	c.Set("X-Request-ID", requestID)

	if err := c.Next(); err != nil {
		if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
			return handlerErr
		}
	}

	status := c.Response().StatusCode()
	fields := []zap.Field{
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.Int("status", status),
		zap.Duration("latency", time.Since(startedAt)),
		zap.String("ip", c.IP()),
		zap.String("request_id", requestID),
	}
	switch {
	case status >= fiber.StatusInternalServerError:
		m.log.Error("http request", fields...)
	case status >= fiber.StatusBadRequest:
		m.log.Warn("http request", fields...)
	default:
		m.log.Info("http request", fields...)
	}
	return nil
}

func newRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
