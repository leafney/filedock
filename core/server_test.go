package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/i18n"
	"github.com/leafney/filedock/pkg/zlogx"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
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
	catalog, err := i18n.NewCatalog()
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	server, err := NewServer(config.Default(), log, catalog, versionAPI)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func TestServerVersionRoute(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name        string
		header      string
		wantLocale  string
		wantMessage string
	}{
		{name: "default Chinese", wantLocale: "zh-CN", wantMessage: "操作成功"},
		{name: "English", header: "en", wantLocale: "en", wantMessage: "Success"},
		{name: "English region", header: "en-US", wantLocale: "en", wantMessage: "Success"},
		{name: "weighted", header: "zh-CN;q=0.4, en-US;q=0.9", wantLocale: "en", wantMessage: "Success"},
		{name: "unsupported", header: "fr", wantLocale: "zh-CN", wantMessage: "操作成功"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(fiber.MethodGet, "/version", nil)
			if tt.header != "" {
				request.Header.Set(fiber.HeaderAcceptLanguage, tt.header)
			}
			response, err := server.App().Test(request)
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
			if body.Code != 200 || body.Message != tt.wantMessage {
				t.Fatalf("/version response = (%d, %q), want (200, %q)", body.Code, body.Message, tt.wantMessage)
			}
			if got := response.Header.Get(fiber.HeaderContentLanguage); got != tt.wantLocale {
				t.Fatalf("Content-Language = %q, want %q", got, tt.wantLocale)
			}
			if got := response.Header.Get(fiber.HeaderVary); got != fiber.HeaderAcceptLanguage {
				t.Fatalf("Vary = %q, want %q", got, fiber.HeaderAcceptLanguage)
			}
			for key, want := range map[string]string{
				"status": "ok", "version": "test-version", "git_branch": "test-branch",
				"git_commit": "test-commit", "build_time": "test-time",
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
		})
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

func TestServerLocalizesAndSanitizesErrors(t *testing.T) {
	server := newTestServer(t)
	server.App().Get("/api/internal-test", func(*fiber.Ctx) error {
		return errors.New("database password leaked")
	})

	tests := []struct {
		name        string
		method      string
		path        string
		language    string
		wantStatus  int
		wantCode    int
		wantMessage string
	}{
		{name: "English not found", method: fiber.MethodGet, path: "/api/missing", language: "en", wantStatus: 404, wantCode: 40401, wantMessage: "The requested resource does not exist or is unavailable"},
		{name: "English method not allowed", method: fiber.MethodPost, path: "/version", language: "en", wantStatus: 405, wantCode: 40501, wantMessage: "The request method is not supported"},
		{name: "Chinese internal", method: fiber.MethodGet, path: "/api/internal-test", language: "zh-CN", wantStatus: 500, wantCode: 50000, wantMessage: "服务器繁忙，请稍后重试"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Header.Set(fiber.HeaderAcceptLanguage, tt.language)
			response, err := server.App().Test(request)
			if err != nil {
				t.Fatalf("request %s error = %v", tt.path, err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			var body struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode || body.Message != tt.wantMessage {
				t.Fatalf("response = (%d, %q), want (%d, %q)", body.Code, body.Message, tt.wantCode, tt.wantMessage)
			}
			if strings.Contains(body.Message, "password") {
				t.Fatalf("response leaked internal error: %q", body.Message)
			}
		})
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
