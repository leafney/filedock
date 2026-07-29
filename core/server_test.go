package core

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/zlogx"
)

func TestServerVersionRoute(t *testing.T) {
	versionSvc := service.NewVersionSvc(service.BuildInfo{
		Version:   "test-version",
		Branch:    "test-branch",
		Commit:    "test-commit",
		BuildTime: "test-time",
	})
	versionBiz, err := biz.NewVersionBiz(versionSvc)
	if err != nil {
		t.Fatalf("NewVersionBiz() error = %v", err)
	}
	versionAPI, err := api.NewVersionAPI(versionBiz)
	if err != nil {
		t.Fatalf("NewVersionAPI() error = %v", err)
	}
	log := zlogx.NewZLogSvcWithConfig(zlogx.Config{Enable: false, Level: "info", Output: "stdout"}, nil)
	server, err := NewServer(config.Default(), log, versionAPI)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	response, err := server.App().Test(httptest.NewRequest(fiber.MethodGet, "/version", nil))
	if err != nil {
		t.Fatalf("request /version error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("/version status = %d, want 200", response.StatusCode)
	}
	var body struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode /version response: %v", err)
	}
	if body.Code != 200 {
		t.Fatalf("/version code = %d, want 200", body.Code)
	}
	if body.Message != "success" {
		t.Fatalf("/version message = %q, want success", body.Message)
	}
	for key, want := range map[string]string{
		"status":     "ok",
		"version":    "test-version",
		"git_branch": "test-branch",
		"git_commit": "test-commit",
		"build_time": "test-time",
	} {
		if body.Data[key] != want {
			t.Fatalf("/version data.%s = %q, want %q", key, body.Data[key], want)
		}
	}
	if _, ok := body.Data["service"]; ok {
		t.Fatal("/version response data contains forbidden service field")
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is empty")
	}

	healthResponse, err := server.App().Test(httptest.NewRequest(fiber.MethodGet, "/health", nil))
	if err != nil {
		t.Fatalf("request /health error = %v", err)
	}
	defer healthResponse.Body.Close()
	if healthResponse.StatusCode == fiber.StatusOK {
		t.Fatal("/health unexpectedly returned 200")
	}
}

func TestRegisterStaticFS(t *testing.T) {
	app := fiber.New()
	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>FileDock</html>")},
		"assets/app.js": {Data: []byte("console.log('filedock')")},
	}
	if err := registerStaticFS(app, dist); err != nil {
		t.Fatalf("registerStaticFS() error = %v", err)
	}

	assetResponse, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/assets/app.js", nil))
	if err != nil {
		t.Fatalf("request asset error = %v", err)
	}
	defer assetResponse.Body.Close()
	if assetResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("asset status = %d, want 200", assetResponse.StatusCode)
	}

	spaResponse, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/rooms/1234", nil))
	if err != nil {
		t.Fatalf("request SPA route error = %v", err)
	}
	defer spaResponse.Body.Close()
	if spaResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("SPA status = %d, want 200", spaResponse.StatusCode)
	}
	content, err := io.ReadAll(spaResponse.Body)
	if err != nil {
		t.Fatalf("read SPA response: %v", err)
	}
	if string(content) != "<html>FileDock</html>" {
		t.Fatalf("SPA content = %q", content)
	}

	apiResponse, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/test", nil))
	if err != nil {
		t.Fatalf("request API route error = %v", err)
	}
	defer apiResponse.Body.Close()
	if apiResponse.StatusCode == fiber.StatusOK {
		t.Fatal("API route unexpectedly received SPA fallback")
	}
}
