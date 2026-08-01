package core

import (
	"bytes"
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
	"github.com/leafney/filedock/internal/dal"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/i18n"
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	return newTestServerWithConfig(t, cfg)
}

func newTestServerWithConfig(t *testing.T, cfg *config.Config) *Server {
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
	dsn := "file:core_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test sqlite: %v", err)
	}
	if err := dal.AutoMigrate(db); err != nil {
		t.Fatalf("migrate test sqlite: %v", err)
	}
	nicknameSvc, err := service.NewNicknameSvc(db)
	if err != nil {
		t.Fatalf("NewNicknameSvc() error = %v", err)
	}
	sessionSvc, err := service.NewSessionSvc(db, cfg.App.DataDir, nicknameSvc)
	if err != nil {
		t.Fatalf("NewSessionSvc() error = %v", err)
	}
	sessionBiz, err := biz.NewSessionBiz(sessionSvc, nicknameSvc)
	if err != nil {
		t.Fatalf("NewSessionBiz() error = %v", err)
	}
	sessionAPI, err := api.NewSessionAPI(sessionBiz, cfg)
	if err != nil {
		t.Fatalf("NewSessionAPI() error = %v", err)
	}
	hub := service.NewStreamHub()
	presence, err := service.NewPresenceSvc(db, hub)
	if err != nil {
		t.Fatalf("NewPresenceSvc() error = %v", err)
	}
	streamAPI, err := api.NewStreamAPI(hub, presence)
	if err != nil {
		t.Fatalf("NewStreamAPI() error = %v", err)
	}
	roomSvc, err := service.NewRoomSvc(db, hub, presence)
	if err != nil {
		t.Fatalf("NewRoomSvc() error = %v", err)
	}
	roomBiz, err := biz.NewRoomBiz(roomSvc)
	if err != nil {
		t.Fatalf("NewRoomBiz() error = %v", err)
	}
	roomAPI, err := api.NewRoomAPI(roomBiz)
	if err != nil {
		t.Fatalf("NewRoomAPI() error = %v", err)
	}
	storage, err := service.NewFileStorage(cfg.App.DataDir)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	fileSvc, err := service.NewFileSvc(db, hub, storage)
	if err != nil {
		t.Fatalf("NewFileSvc() error = %v", err)
	}
	fileBiz, err := biz.NewFileBiz(fileSvc)
	if err != nil {
		t.Fatalf("NewFileBiz() error = %v", err)
	}
	fileAPI, err := api.NewFileAPI(fileBiz)
	if err != nil {
		t.Fatalf("NewFileAPI() error = %v", err)
	}
	server, err := NewServer(cfg, log, catalog, versionAPI, sessionAPI, sessionSvc, roomAPI, fileAPI, streamAPI, service.NewRateLimiter())
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

func TestServerTLSConfigurationAndSecureCookie(t *testing.T) {
	if err := validateTLSConfig(config.HTTPConfig{CertFile: "cert.pem"}); err == nil {
		t.Fatal("partial TLS configuration unexpectedly accepted")
	}
	if err := validateTLSConfig(config.HTTPConfig{KeyFile: "key.pem"}); err == nil {
		t.Fatal("partial TLS key configuration unexpectedly accepted")
	}

	cfg := config.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.HTTP.CertFile = "cert.pem"
	cfg.HTTP.KeyFile = "key.pem"
	server := newTestServerWithConfig(t, cfg)
	request := httptest.NewRequest(fiber.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"displayName":"TLS用户"}`))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	response, err := server.App().Test(request)
	if err != nil {
		t.Fatalf("create session request error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("create session status = %d, want 200", response.StatusCode)
	}
	setCookie := response.Header.Get(fiber.HeaderSetCookie)
	if !strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Fatalf("HTTPS session cookie missing Secure attribute: %q", setCookie)
	}
}

func TestServerFileUploadBatchAndStreamingContent(t *testing.T) {
	server := newTestServer(t)
	createSession := httptest.NewRequest(fiber.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"displayName":"文件用户"}`))
	createSession.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	sessionResponse, err := server.App().Test(createSession)
	if err != nil {
		t.Fatal(err)
	}
	defer sessionResponse.Body.Close()
	cookie := sessionResponse.Header.Get(fiber.HeaderSetCookie)
	if cookie == "" {
		t.Fatal("session cookie is empty")
	}
	createRoom := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms", bytes.NewBufferString(`{"joinMode":"open"}`))
	createRoom.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	createRoom.Header.Set(fiber.HeaderCookie, cookie)
	roomResponse, err := server.App().Test(createRoom)
	if err != nil {
		t.Fatal(err)
	}
	defer roomResponse.Body.Close()
	var roomBody struct {
		Data struct {
			RoomCode string `json:"roomCode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(roomResponse.Body).Decode(&roomBody); err != nil || roomBody.Data.RoomCode == "" {
		t.Fatalf("decode room response: code=%q error=%v", roomBody.Data.RoomCode, err)
	}
	manifest := `{"idempotencyKey":"integration-upload","scope":"shared","files":[{"originalName":"hello.txt","declaredSize":5,"declaredMime":"text/plain"}]}`
	createBatch := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/file-upload-batches", bytes.NewBufferString(manifest))
	createBatch.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	createBatch.Header.Set(fiber.HeaderCookie, cookie)
	batchResponse, err := server.App().Test(createBatch)
	if err != nil {
		t.Fatal(err)
	}
	defer batchResponse.Body.Close()
	if batchResponse.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(batchResponse.Body)
		t.Fatalf("create batch status=%d body=%s", batchResponse.StatusCode, body)
	}
	var batchBody struct {
		Data struct {
			Files []struct {
				FileID string `json:"fileId"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(batchResponse.Body).Decode(&batchBody); err != nil || len(batchBody.Data.Files) != 1 {
		t.Fatalf("decode batch response: %+v error=%v", batchBody, err)
	}
	upload := httptest.NewRequest(fiber.MethodPut, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/files/"+batchBody.Data.Files[0].FileID+"/content", bytes.NewBufferString("hello"))
	upload.Header.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	upload.Header.Set(fiber.HeaderCookie, cookie)
	uploadResponse, err := server.App().Test(upload, 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer uploadResponse.Body.Close()
	if uploadResponse.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(uploadResponse.Body)
		t.Fatalf("upload status=%d body=%s", uploadResponse.StatusCode, body)
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

func TestSessionRoutesUseCookieIdentity(t *testing.T) {
	server := newTestServer(t)
	createRequest := httptest.NewRequest(fiber.MethodPost, "/api/v1/sessions", strings.NewReader(`{"displayName":"张飞"}`))
	createRequest.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	createResponse, err := server.App().Test(createRequest)
	if err != nil {
		t.Fatalf("create session request error = %v", err)
	}
	defer createResponse.Body.Close()
	if createResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("create session status = %d, want 200", createResponse.StatusCode)
	}
	cookie := createResponse.Header.Get(fiber.HeaderSetCookie)
	if !strings.HasPrefix(cookie, "session_token=") || !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("session cookie = %q, missing required attributes", cookie)
	}
	var created struct {
		Data struct {
			UserID      string `json:"userId"`
			DisplayName string `json:"displayName"`
		} `json:"data"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode create session response: %v", err)
	}
	if created.Data.UserID == "" || created.Data.DisplayName != "张飞" {
		t.Fatalf("created session data = %+v", created.Data)
	}

	currentRequest := httptest.NewRequest(fiber.MethodGet, "/api/v1/sessions/me", nil)
	currentRequest.Header.Set(fiber.HeaderCookie, strings.Split(cookie, ";")[0])
	currentResponse, err := server.App().Test(currentRequest)
	if err != nil {
		t.Fatalf("current session request error = %v", err)
	}
	defer currentResponse.Body.Close()
	if currentResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("current session status = %d, want 200", currentResponse.StatusCode)
	}

	resetRequest := httptest.NewRequest(fiber.MethodDelete, "/api/v1/sessions/me", nil)
	resetRequest.Header.Set(fiber.HeaderCookie, strings.Split(cookie, ";")[0])
	resetResponse, err := server.App().Test(resetRequest)
	if err != nil {
		t.Fatalf("reset session request error = %v", err)
	}
	defer resetResponse.Body.Close()
	if resetResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("reset session status = %d, want 200", resetResponse.StatusCode)
	}
	resetCookie := resetResponse.Header.Get(fiber.HeaderSetCookie)
	if !strings.HasPrefix(resetCookie, "session_token=;") {
		t.Fatalf("reset cookie = %q, want empty session cookie", resetCookie)
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
