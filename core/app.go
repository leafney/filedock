package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/pkg/gormx"
	"github.com/leafney/filedock/pkg/zlogx"
)

type App struct {
	cfg    *config.Config
	log    *zlogx.ZLogSvc
	db     *gormx.GormDBSvc
	server *Server

	closeOnce sync.Once
	closeErr  error
}

func NewApp(cfg *config.Config, log *zlogx.ZLogSvc, db *gormx.GormDBSvc, server *Server) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("app config is required")
	}
	if log == nil {
		return nil, fmt.Errorf("app logger is required")
	}
	if db == nil {
		return nil, fmt.Errorf("app database is required")
	}
	if server == nil {
		return nil, fmt.Errorf("app server is required")
	}
	return &App{cfg: cfg, log: log, db: db, server: server}, nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.RunContext(ctx)
}

func (a *App) RunContext(ctx context.Context) (err error) {
	if a == nil {
		return fmt.Errorf("app is nil")
	}
	if ctx == nil {
		return fmt.Errorf("app context is required")
	}
	a.log.Info("filedock starting")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- a.server.Run()
	}()
	defer func() {
		if closeErr := a.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case serverErrValue := <-serverErr:
		if serverErrValue != nil {
			return fmt.Errorf("http server stopped: %w", serverErrValue)
		}
		return nil
	}
}

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		var errs []error
		if err := a.server.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
		}
		if err := a.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sqlite: %w", err))
		}
		if err := a.log.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) && !errors.Is(err, syscall.EBADF) {
			errs = append(errs, fmt.Errorf("sync logger: %w", err))
		}
		a.closeErr = errors.Join(errs...)
	})
	return a.closeErr
}
