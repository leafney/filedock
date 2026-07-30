package core

import (
	"fmt"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
)

func rateLimited(limiter *service.RateLimiter, action string, limit int, window time.Duration, handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if limiter == nil {
			return handler(c)
		}
		allowed, retryAfter := limiter.Allow(action, service.RemoteIP(c.Context().RemoteAddr().String()), limit, window)
		if !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			c.Set(fiber.HeaderRetryAfter, fmt.Sprintf("%d", seconds))
			return response.Error(c, errc.ErrRateLimited, nil)
		}
		return handler(c)
	}
}
