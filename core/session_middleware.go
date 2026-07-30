package core

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/service"
)

func sessionMiddleware(sessionSvc *service.SessionSvc, secure bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if sessionSvc == nil {
			return c.Next()
		}
		token := c.Cookies(service.SessionCookieName)
		if token == "" {
			return c.Next()
		}
		principal, err := sessionSvc.Authenticate(token)
		if err == nil {
			c.Locals(service.SessionContextKey, principal)
			if principal.RefreshedToken != "" {
				setSessionCookie(c, principal.RefreshedToken, principal.SessionExpiresAt, secure)
			}
		}
		return c.Next()
	}
}

func setSessionCookie(c *fiber.Ctx, token string, expiresAt int64, secure bool) {
	maxAge := int(expiresAt - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	c.Cookie(&fiber.Cookie{Name: service.SessionCookieName, Value: token, Path: "/", HTTPOnly: true, Secure: secure, SameSite: "Lax", MaxAge: maxAge})
}
