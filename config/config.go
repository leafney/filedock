package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultConfigPath = "data/config.toml"
	DefaultDataDir    = "data"
	DefaultHTTPAddr   = ":8195"
)

type Config struct {
	App    AppConfig    `toml:"app"`
	HTTP   HTTPConfig   `toml:"http"`
	Log    LogConfig    `toml:"log"`
	SQLite SQLiteConfig `toml:"sqlite"`
}

type AppConfig struct {
	Name    string `toml:"name"`
	Env     string `toml:"env"`
	DataDir string `toml:"data_dir"`
}

type HTTPConfig struct {
	Addr     string `toml:"addr"`
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

func (c HTTPConfig) TLS() bool {
	return strings.TrimSpace(c.CertFile) != "" || strings.TrimSpace(c.KeyFile) != ""
}

type LogConfig struct {
	Enable bool   `toml:"enable"`
	Level  string `toml:"level"`
	Output string `toml:"output"`
	File   string `toml:"file"`
	Caller bool   `toml:"caller"`
}

type SQLiteConfig struct {
	Path string `toml:"path"`
}

func Default() *Config {
	return &Config{
		App: AppConfig{
			Name:    "filedock",
			Env:     "dev",
			DataDir: DefaultDataDir,
		},
		HTTP: HTTPConfig{Addr: DefaultHTTPAddr},
		Log: LogConfig{
			Enable: true,
			Level:  "info",
			Output: "stdout",
			File:   "data/logs/filedock.log",
			Caller: true,
		},
	}
}

func DefaultPath() string {
	return DefaultConfigPath
}

func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}

	cfg := Default()
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("decode config %q: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat config %q: %w", path, err)
	}

	if err := cfg.prepare(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) prepare() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.App.Name == "" {
		c.App.Name = "filedock"
	}
	if c.App.Env == "" {
		c.App.Env = "dev"
	}
	if c.App.DataDir == "" {
		c.App.DataDir = DefaultDataDir
	}
	if c.HTTP.Addr == "" {
		c.HTTP.Addr = DefaultHTTPAddr
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Output == "" {
		c.Log.Output = "stdout"
	}
	if c.Log.File == "" {
		c.Log.File = filepath.Join(c.App.DataDir, "logs", "filedock.log")
	}
	if c.SQLite.Path == "" {
		c.SQLite.Path = filepath.Join(c.App.DataDir, "filedock.db")
	}

	if err := os.MkdirAll(c.App.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.SQLite.Path), 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.Log.File), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	return nil
}
