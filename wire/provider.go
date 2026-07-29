package wire

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/core"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/gormx"
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

const sqliteBusyTimeout = 5000

func provideConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

func provideLogger(cfg *config.Config) *zlogx.ZLogSvc {
	return zlogx.NewZLogSvcWithConfig(zlogx.Config{
		Enable: cfg.Log.Enable,
		Level:  cfg.Log.Level,
		Caller: cfg.Log.Caller,
		Output: cfg.Log.Output,
		File:   cfg.Log.File,
	}, nil)
}

func provideSQLiteDB(cfg *config.Config, log *zlogx.ZLogSvc) (*gormx.GormDBSvc, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sqlite config is required")
	}
	if log == nil {
		return nil, fmt.Errorf("sqlite logger is required")
	}
	if strings.TrimSpace(cfg.SQLite.Path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	dsn := buildSQLiteDSN(cfg.SQLite.Path)
	db, err := gormx.NewGormDBSvc(gormx.Options{
		Dialector: sqlite.Open(dsn),
		Retry: gormx.RetryOptions{
			Enabled: false,
		},
		OnOpen: validateSQLitePragmas,
		AfterOpen: func(sqlDB *sql.DB) {
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetMaxIdleConns(1)
		},
	}, log)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

func buildSQLiteDSN(path string) string {
	parts := []string{
		"_txlock=immediate",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(1)",
		fmt.Sprintf("_pragma=busy_timeout(%d)", sqliteBusyTimeout),
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + strings.Join(parts, "&")
}

func validateSQLitePragmas(db *gorm.DB) error {
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		return fmt.Errorf("read sqlite journal mode: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(journalMode), "wal") {
		return fmt.Errorf("sqlite journal mode = %q, want WAL", journalMode)
	}

	var foreignKeys int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		return fmt.Errorf("read sqlite foreign keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("sqlite foreign keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		return fmt.Errorf("read sqlite busy timeout: %w", err)
	}
	if busyTimeout < sqliteBusyTimeout {
		return fmt.Errorf("sqlite busy timeout = %d, want at least %d", busyTimeout, sqliteBusyTimeout)
	}
	return nil
}

func provideVersionSvc(build core.BuildInfo) *service.VersionSvc {
	return service.NewVersionSvc(service.BuildInfo{
		Version:   build.Version,
		Branch:    build.Branch,
		Commit:    build.Commit,
		BuildTime: build.BuildTime,
	})
}

func provideVersionBiz(version *service.VersionSvc) (*biz.VersionBiz, error) {
	return biz.NewVersionBiz(version)
}

func provideVersionAPI(versionBiz *biz.VersionBiz) (*api.VersionAPI, error) {
	return api.NewVersionAPI(versionBiz)
}

func provideServer(cfg *config.Config, log *zlogx.ZLogSvc, versionAPI *api.VersionAPI) (*core.Server, error) {
	return core.NewServer(cfg, log, versionAPI)
}

func provideApp(cfg *config.Config, log *zlogx.ZLogSvc, db *gormx.GormDBSvc, server *core.Server) (*core.App, error) {
	return core.NewApp(cfg, log, db, server)
}
