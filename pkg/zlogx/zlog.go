package zlogx

import (
	"errors"
	"syscall"

	rzap "github.com/leafney/rose-zap"
)

type ZLogSvc struct {
	*rzap.Logger
}

type Config struct {
	Enable bool
	Level  string
	Caller bool
	Output string
	File   string
}

// NewZLogSvc 创建 Zap 日志服务（Server 端使用）
// enable: 是否启用
// level: 日志级别
// caller: 是否显示调用者信息
// stop: 停止信号通道
func NewZLogSvc(enable bool, level string, caller bool, stop chan struct{}) *ZLogSvc {
	return NewZLogSvcWithConfig(Config{
		Enable: enable,
		Level:  level,
		Caller: caller,
		Output: "file",
	}, stop)
}

func NewZLogSvcWithConfig(cfg Config, stop chan struct{}) *ZLogSvc {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Output == "" {
		cfg.Output = "stdout"
	}

	zc := rzap.NewConfig()

	zc.
		SetEnable(cfg.Enable).
		SetLevel(cfg.Level).
		ShowCaller(cfg.Caller).
		ShowStacktrace(false)

	switch cfg.Output {
	case "file":
		zc.OutSingleFile(false)
		if cfg.File != "" {
			zc.SetFileConfig(rzap.WithFileName(cfg.File))
		}
	default:
		// rose-zap defaults to stdout.
	}

	logger := rzap.NewLogger(zc)

	// 监听停止信号，优雅关闭日志
	if stop != nil {
		go func() {
			<-stop
			if err := logger.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
				// 无法使用 logger，因为可能已经关闭
				return
			}
		}()
	}

	return &ZLogSvc{logger}
}

// NewDefaultZLogSvc 创建默认的 Zap 日志服务
func NewDefaultZLogSvc() *ZLogSvc {
	return NewZLogSvc(true, "info", true, nil)
}
