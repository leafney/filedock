package gormx

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/leafney/filedock/pkg/zlogx"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type GormDBSvc struct {
	*gorm.DB
	sqlDB *sql.DB
	log   *zlogx.ZLogSvc
}

type RetryOptions struct {
	Enabled        bool
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JitterMax      time.Duration
}

type Options struct {
	Dialector gorm.Dialector
	Debug     bool
	Retry     RetryOptions
	OnOpen    func(*gorm.DB) error
	AfterOpen func(*sql.DB)
}

func defaultRetryOptions() RetryOptions {
	return RetryOptions{
		Enabled:        true,
		MaxAttempts:    10,
		InitialBackoff: time.Second,
		MaxBackoff:     30 * time.Second,
		JitterMax:      100 * time.Millisecond,
	}
}

func open(opts Options, log *zlogx.ZLogSvc) (*gorm.DB, *sql.DB, error) {
	if opts.Dialector == nil {
		return nil, nil, fmt.Errorf("gorm dialector is required")
	}

	retry := opts.Retry
	if retry.MaxAttempts <= 0 {
		retry = defaultRetryOptions()
	}
	if !retry.Enabled {
		return openOnce(opts, log)
	}

	backoff := retry.InitialBackoff
	if backoff <= 0 {
		backoff = time.Second
	}
	maxBackoff := retry.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}
	jitterMax := retry.JitterMax
	if jitterMax < 0 {
		jitterMax = 0
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var lastErr error
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		db, sqlDB, err := openOnce(opts, log)
		if err == nil {
			return db, sqlDB, nil
		}
		lastErr = err
		if attempt == retry.MaxAttempts {
			break
		}

		sleep := backoff
		if jitterMax > 0 {
			sleep += time.Duration(rng.Int63n(int64(jitterMax) + 1))
		}
		log.Error(fmt.Sprintf("[Gorm] connect attempt %d/%d failed [%v], retrying in %s", attempt, retry.MaxAttempts, lastErr, sleep))
		time.Sleep(sleep)

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return nil, nil, fmt.Errorf("gorm connect failed after %d attempts: %w", retry.MaxAttempts, lastErr)
}

func openOnce(opts Options, log *zlogx.ZLogSvc) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(opts.Dialector, &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}
	if opts.Debug {
		db = db.Debug()
		log.Info("[Gorm] Debug mode enabled")
	} else {
		db.Logger = logger.Default.LogMode(logger.Silent)
	}

	if opts.OnOpen != nil {
		if err := opts.OnOpen(db); err != nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr == nil {
				_ = sqlDB.Close()
			}
			return nil, nil, err
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}
	if opts.AfterOpen != nil {
		opts.AfterOpen(sqlDB)
	}
	return db, sqlDB, nil
}

func NewGormDBSvc(opts Options, log *zlogx.ZLogSvc) (*GormDBSvc, error) {
	db, sqlDB, err := open(opts, log)
	if err != nil {
		return nil, err
	}
	log.Info("[Gorm] Load successful")
	return &GormDBSvc{
		DB:    db,
		sqlDB: sqlDB,
		log:   log,
	}, nil
}

func (s *GormDBSvc) Close() error {
	if s == nil || s.sqlDB == nil {
		return nil
	}
	if err := s.sqlDB.Close(); err != nil {
		return err
	}
	if s.log != nil {
		s.log.Info("[Gorm] Exit successful")
	}
	return nil
}
