package wire

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/core"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dal"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/gormx"
	"github.com/leafney/filedock/pkg/i18n"
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/libtnb/sqlite"
	"go.uber.org/zap"
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

func provideI18nCatalog(log *zlogx.ZLogSvc) (*i18n.Catalog, error) {
	if log == nil {
		return nil, fmt.Errorf("i18n logger is required")
	}
	return i18n.NewCatalog(i18n.WithMissingKeyHandler(func(locale i18n.Locale, key string, err error) {
		log.Error("missing translation",
			zap.String("locale", string(locale)),
			zap.String("key", key),
			zap.Error(err),
		)
	}))
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
	if err := dal.AutoMigrate(db.DB); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auto migrate database: %w", err)
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

func provideNicknameSvc(db *gormx.GormDBSvc) (*service.NicknameSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("nickname database is required")
	}
	return service.NewNicknameSvc(db.DB)
}

func provideSessionSvc(cfg *config.Config, db *gormx.GormDBSvc, nickname *service.NicknameSvc, hub *service.StreamHub) (*service.SessionSvc, error) {
	if cfg == nil {
		return nil, fmt.Errorf("session config is required")
	}
	if db == nil {
		return nil, fmt.Errorf("session database is required")
	}
	return service.NewSessionSvc(db.DB, cfg.App.DataDir, nickname, hub)
}

func provideSessionBiz(session *service.SessionSvc, nickname *service.NicknameSvc) (*biz.SessionBiz, error) {
	return biz.NewSessionBiz(session, nickname)
}

func provideSessionAPI(sessionBiz *biz.SessionBiz, cfg *config.Config) (*api.SessionAPI, error) {
	return api.NewSessionAPI(sessionBiz, cfg)
}

func provideRoomSvc(db *gormx.GormDBSvc, hub *service.StreamHub, presence *service.PresenceSvc, files *service.FileSvc) (*service.RoomSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("room database is required")
	}
	room, err := service.NewRoomSvc(db.DB, hub, presence)
	if err == nil {
		room.AttachFileService(files)
	}
	return room, err
}

func provideRoomBiz(room *service.RoomSvc) (*biz.RoomBiz, error) {
	return biz.NewRoomBiz(room)
}

func provideRoomAPI(roomBiz *biz.RoomBiz) (*api.RoomAPI, error) {
	return api.NewRoomAPI(roomBiz)
}

func provideFileSvc(db *gormx.GormDBSvc, hub *service.StreamHub, storage *service.FileStorage) (*service.FileSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("file database is required")
	}
	return service.NewFileSvc(db.DB, hub, storage)
}

func provideFileBiz(files *service.FileSvc) (*biz.FileBiz, error) {
	return biz.NewFileBiz(files)
}

func provideFileAPI(fileBiz *biz.FileBiz) (*api.FileAPI, error) {
	return api.NewFileAPI(fileBiz)
}

func provideChatSvc(db *gormx.GormDBSvc, hub *service.StreamHub, limiter *service.RateLimiter, presence *service.PresenceSvc) (*service.ChatSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("chat database is required")
	}
	return service.NewChatSvc(db.DB, hub, limiter, presence)
}

func provideChatBiz(chat *service.ChatSvc) (*biz.ChatBiz, error) {
	return biz.NewChatBiz(chat)
}

func provideChatAPI(chatBiz *biz.ChatBiz) (*api.ChatAPI, error) {
	return api.NewChatAPI(chatBiz)
}

func provideNotificationSvc(db *gormx.GormDBSvc) (*service.NotificationSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("notification database is required")
	}
	return service.NewNotificationSvc(db.DB)
}

func provideNotificationBiz(notifications *service.NotificationSvc) (*biz.NotificationBiz, error) {
	return biz.NewNotificationBiz(notifications)
}

func provideNotificationAPI(notificationBiz *biz.NotificationBiz) (*api.NotificationAPI, error) {
	return api.NewNotificationAPI(notificationBiz)
}

func provideStreamHub() *service.StreamHub {
	return service.NewStreamHub()
}

func providePresenceSvc(db *gormx.GormDBSvc, hub *service.StreamHub) (*service.PresenceSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("presence database is required")
	}
	return service.NewPresenceSvc(db.DB, hub)
}

func provideStreamAPI(hub *service.StreamHub, presence *service.PresenceSvc) (*api.StreamAPI, error) {
	return api.NewStreamAPI(hub, presence)
}

func provideFileStorage(cfg *config.Config) (*service.FileStorage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("file storage config is required")
	}
	return service.NewFileStorage(cfg.App.DataDir)
}

func provideLifecycleSvc(db *gormx.GormDBSvc, hub *service.StreamHub, storage *service.FileStorage, files *service.FileSvc) (*service.LifecycleSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("lifecycle database is required")
	}
	lifecycle, err := service.NewLifecycleSvc(db.DB, hub, storage)
	if err == nil {
		lifecycle.AttachFileService(files)
	}
	return lifecycle, err
}

func provideRateLimiter() *service.RateLimiter {
	return service.NewRateLimiter()
}

func provideServer(cfg *config.Config, log *zlogx.ZLogSvc, catalog *i18n.Catalog, versionAPI *api.VersionAPI, sessionAPI *api.SessionAPI, sessionSvc *service.SessionSvc, roomAPI *api.RoomAPI, fileAPI *api.FileAPI, streamAPI *api.StreamAPI, limiter *service.RateLimiter, chatAPI *api.ChatAPI, notificationAPI *api.NotificationAPI) (*core.Server, error) {
	return core.NewServer(cfg, log, catalog, versionAPI, sessionAPI, sessionSvc, roomAPI, fileAPI, streamAPI, limiter, chatAPI, notificationAPI)
}

func provideApp(cfg *config.Config, log *zlogx.ZLogSvc, db *gormx.GormDBSvc, lifecycle *service.LifecycleSvc, server *core.Server) (*core.App, error) {
	return core.NewApp(cfg, log, db, lifecycle, server)
}
