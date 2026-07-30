package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/i18n"
	"github.com/leafney/filedock/pkg/response"
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/leafney/filedock/static"
	"go.uber.org/zap"
)

type Server struct {
	cfg *config.Config
	app *fiber.App
}

func NewServer(cfg *config.Config, log *zlogx.ZLogSvc, catalog *i18n.Catalog, versionAPI *api.VersionAPI, sessionAPI *api.SessionAPI, sessionSvc *service.SessionSvc) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server config is required")
	}
	if log == nil {
		return nil, fmt.Errorf("server logger is required")
	}
	if catalog == nil {
		return nil, fmt.Errorf("server i18n catalog is required")
	}
	if versionAPI == nil {
		return nil, fmt.Errorf("version api is required")
	}
	if sessionAPI == nil {
		return nil, fmt.Errorf("session api is required")
	}
	if sessionSvc == nil {
		return nil, fmt.Errorf("session service is required")
	}
	if (cfg.HTTP.CertFile == "") != (cfg.HTTP.KeyFile == "") {
		return nil, fmt.Errorf("tls certificate and key must be configured together")
	}

	requestLogger, err := NewRequestLogger(log)
	if err != nil {
		return nil, err
	}
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return handleHTTPError(log, c, err)
	}})
	app.Use(recover.New())
	app.Use(requestLogger.Handle)
	app.Use(localizationMiddleware(catalog))
	app.Use(sessionMiddleware(sessionSvc, cfg.HTTP.CertFile != "" && cfg.HTTP.KeyFile != ""))
	registerRoutes(app, versionAPI, sessionAPI)
	if err := registerStatic(app); err != nil {
		return nil, fmt.Errorf("register static resources: %w", err)
	}
	return &Server{cfg: cfg, app: app}, nil
}

func (s *Server) Run() error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if s.cfg.HTTP.CertFile != "" && s.cfg.HTTP.KeyFile != "" {
		return s.app.ListenTLS(s.cfg.HTTP.Addr, s.cfg.HTTP.CertFile, s.cfg.HTTP.KeyFile)
	}
	return s.app.Listen(s.cfg.HTTP.Addr)
}

func (s *Server) App() *fiber.App {
	if s == nil {
		return nil
	}
	return s.app
}

func (s *Server) Shutdown() error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.app.ShutdownWithContext(ctx)
}

func handleHTTPError(log *zlogx.ZLogSvc, c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
	}
	if status >= fiber.StatusInternalServerError && log != nil {
		fields := []zap.Field{
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
		}
		if requestID, ok := c.Locals(requestIDLocalKey).(string); ok {
			fields = append(fields, zap.String("request_id", requestID))
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		log.Error("http handler error", fields...)
	}
	setLanguageResponseHeaders(c, response.Locale(c))
	return response.Error(c, errorCodeForHTTPStatus(status), nil)
}

func errorCodeForHTTPStatus(status int) int {
	switch status {
	case fiber.StatusBadRequest:
		return errc.ErrClient
	case fiber.StatusMethodNotAllowed:
		return errc.ErrMethodNotAllowed
	case fiber.StatusUnauthorized:
		return errc.ErrUnAuthorized
	case fiber.StatusForbidden:
		return errc.ErrForbidden
	case fiber.StatusNotFound:
		return errc.ErrNotFound
	case fiber.StatusRequestTimeout, fiber.StatusGatewayTimeout:
		return errc.ErrTimeOut
	case fiber.StatusConflict:
		return errc.ErrConflict
	default:
		return errc.ErrServer
	}
}

func registerStatic(app *fiber.App) error {
	dist, err := static.Dist()
	if err != nil {
		return err
	}
	return registerStaticFS(app, dist)
}

func registerStaticFS(app *fiber.App, dist fs.FS) error {
	if app == nil {
		return fmt.Errorf("static app is required")
	}
	if dist == nil {
		return fmt.Errorf("static filesystem is required")
	}
	app.Use("/", filesystem.New(filesystem.Config{
		Root:  http.FS(dist),
		Index: "index.html",
	}))
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet ||
			c.Path() == "/version" ||
			c.Path() == "/health" ||
			strings.HasPrefix(c.Path(), "/api/") ||
			strings.HasPrefix(c.Path(), "/open/") ||
			strings.HasPrefix(c.Path(), "/public/") {
			return c.Next()
		}

		data, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		c.Type("html")
		return c.Status(fiber.StatusOK).Send(data)
	})
	return nil
}
