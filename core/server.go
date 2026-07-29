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
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/leafney/filedock/static"
)

type Server struct {
	cfg *config.Config
	app *fiber.App
}

func NewServer(cfg *config.Config, log *zlogx.ZLogSvc, versionAPI *api.VersionAPI) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server config is required")
	}
	if log == nil {
		return nil, fmt.Errorf("server logger is required")
	}
	if versionAPI == nil {
		return nil, fmt.Errorf("version api is required")
	}

	requestLogger, err := NewRequestLogger(log)
	if err != nil {
		return nil, err
	}
	app := fiber.New(fiber.Config{ErrorHandler: handleHTTPError})
	app.Use(recover.New())
	app.Use(requestLogger.Handle)
	registerRoutes(app, versionAPI)
	if err := registerStatic(app); err != nil {
		return nil, fmt.Errorf("register static resources: %w", err)
	}
	return &Server{cfg: cfg, app: app}, nil
}

func (s *Server) Run() error {
	if s == nil {
		return fmt.Errorf("server is nil")
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

func handleHTTPError(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "Internal Server Error"
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
		message = fiberErr.Message
	} else if err != nil {
		message = err.Error()
	}
	if strings.HasPrefix(c.Path(), "/api/") {
		return c.Status(status).JSON(fiber.Map{"code": status, "message": message})
	}
	return c.Status(status).JSON(fiber.Map{"message": message})
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
