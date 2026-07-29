package wire

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leafney/filedock/core"
)

func TestInitializeApp(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	configPath := filepath.Join(tmp, "config.toml")
	content := "[app]\ndata_dir = \"" + dataDir + "\"\n[http]\naddr = \":0\"\n[sqlite]\npath = \"" + filepath.Join(dataDir, "filedock.db") + "\"\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app, err := InitializeApp(configPath, core.BuildInfo{Version: "test"})
	if err != nil {
		t.Fatalf("InitializeApp() error = %v", err)
	}
	if app == nil {
		t.Fatal("InitializeApp() returned nil app")
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
