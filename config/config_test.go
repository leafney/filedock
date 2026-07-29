package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenConfigMissing(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.App.Name != "filedock" {
		t.Fatalf("App.Name = %q, want filedock", cfg.App.Name)
	}
	if cfg.HTTP.Addr != ":8195" {
		t.Fatalf("HTTP.Addr = %q, want :8195", cfg.HTTP.Addr)
	}
	if cfg.SQLite.Path != filepath.Join("data", "filedock.db") {
		t.Fatalf("SQLite.Path = %q, want data/filedock.db", cfg.SQLite.Path)
	}
	if _, err := os.Stat(filepath.Join("data", "logs")); err != nil {
		t.Fatalf("log directory was not created: %v", err)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contents := []byte(`[app]
name = "custom"
env = "test"
data_dir = "runtime"

[http]
addr = "127.0.0.1:9000"

[log]
enable = false
level = "debug"
output = "file"
file = "runtime/logs/custom.log"
caller = false

[sqlite]
path = "runtime/custom.db"
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.App.Name != "custom" || cfg.App.Env != "test" || cfg.App.DataDir != "runtime" {
		t.Fatalf("unexpected app config: %+v", cfg.App)
	}
	if cfg.HTTP.Addr != "127.0.0.1:9000" {
		t.Fatalf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.Log.Enable || cfg.Log.Level != "debug" || cfg.Log.Output != "file" || cfg.Log.Caller {
		t.Fatalf("unexpected log config: %+v", cfg.Log)
	}
	if cfg.SQLite.Path != "runtime/custom.db" {
		t.Fatalf("SQLite.Path = %q", cfg.SQLite.Path)
	}
}

func TestLoadReturnsDecodeError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.toml")
	if err := os.WriteFile(path, []byte("[app\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want decode error")
	}
}
