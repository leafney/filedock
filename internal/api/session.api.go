package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
	validation "github.com/leafney/filedock/pkg/validator"
)

type SessionAPI struct {
	biz    *biz.SessionBiz
	secure bool
}

func NewSessionAPI(sessionBiz *biz.SessionBiz, cfg *config.Config) (*SessionAPI, error) {
	if sessionBiz == nil {
		return nil, fmt.Errorf("session business is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("session config is required")
	}
	if cfg.HTTP.CertFile != "" || cfg.HTTP.KeyFile != "" {
		if cfg.HTTP.CertFile == "" || cfg.HTTP.KeyFile == "" {
			return nil, fmt.Errorf("tls certificate and key must be configured together")
		}
	}
	return &SessionAPI{biz: sessionBiz, secure: cfg.HTTP.CertFile != "" && cfg.HTTP.KeyFile != ""}, nil
}

func (a *SessionAPI) HandleRandomNickname(c *fiber.Ctx) error {
	result, err := a.biz.RandomNickname()
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *SessionAPI) HandleCreate(c *fiber.Ctx) error {
	if principal, ok := Principal(c); ok {
		return response.Success(c, a.biz.View(principal))
	}
	var request dto.CreateSessionRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	if err := validation.Validate(request); err != nil {
		return response.ValidationError(c, err)
	}
	result, err := a.biz.Create(request)
	if err != nil {
		return response.Failed(c, err)
	}
	a.setCookie(c, result.Token, result.Principal.SessionExpiresAt)
	return response.Success(c, a.biz.View(result.Principal))
}

func (a *SessionAPI) HandleCurrent(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	return response.Success(c, a.biz.View(principal))
}

func (a *SessionAPI) HandleUpdate(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.UpdateSessionRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	if err := validation.Validate(request); err != nil {
		return response.ValidationError(c, err)
	}
	updated, err := a.biz.UpdateName(principal.UserID, request)
	if err != nil {
		return response.Failed(c, err)
	}
	updated.SessionExpiresAt = principal.SessionExpiresAt
	return response.Success(c, a.biz.View(updated))
}

func (a *SessionAPI) HandleReset(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Reset(principal.UserID); err != nil {
		return response.Failed(c, err)
	}
	a.clearCookie(c)
	return response.Success(c, nil)
}

func Principal(c *fiber.Ctx) (service.Principal, bool) {
	if c == nil {
		return service.Principal{}, false
	}
	principal, ok := c.Locals(service.SessionContextKey).(service.Principal)
	return principal, ok
}

func (a *SessionAPI) setCookie(c *fiber.Ctx, token string, expiresAt int64) {
	maxAge := int(expiresAt - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	c.Cookie(&fiber.Cookie{Name: service.SessionCookieName, Value: token, Path: "/", HTTPOnly: true, Secure: a.secure, SameSite: "Lax", MaxAge: maxAge})
}

func (a *SessionAPI) clearCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: service.SessionCookieName, Value: "", Path: "/", HTTPOnly: true, Secure: a.secure, SameSite: "Lax", MaxAge: -1})
}
