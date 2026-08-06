package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	limiter := service.NewRateLimiter()
	chatSvc, err := service.NewChatSvc(db, hub, limiter, presence)
	if err != nil {
		t.Fatalf("NewChatSvc() error = %v", err)
	}
	chatBiz, err := biz.NewChatBiz(chatSvc)
	if err != nil {
		t.Fatalf("NewChatBiz() error = %v", err)
	}
	chatAPI, err := api.NewChatAPI(chatBiz)
	if err != nil {
		t.Fatalf("NewChatAPI() error = %v", err)
	}
	notificationSvc, err := service.NewNotificationSvc(db)
	if err != nil {
		t.Fatalf("NewNotificationSvc() error = %v", err)
	}
	notificationBiz, err := biz.NewNotificationBiz(notificationSvc)
	if err != nil {
		t.Fatalf("NewNotificationBiz() error = %v", err)
	}
	notificationAPI, err := api.NewNotificationAPI(notificationBiz)
	if err != nil {
		t.Fatalf("NewNotificationAPI() error = %v", err)
	}
	server, err := NewServer(cfg, log, catalog, versionAPI, sessionAPI, sessionSvc, roomAPI, fileAPI, streamAPI, limiter, chatAPI, notificationAPI)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func TestServerChatRoutes(t *testing.T) {
	server := newTestServer(t)
	type identity struct {
		cookie string
		userID string
	}
	createSession := func(name string) identity {
		t.Helper()
		request := httptest.NewRequest(fiber.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"displayName":"`+name+`"}`))
		request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		response, err := server.App().Test(request)
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		defer response.Body.Close()
		if response.StatusCode != fiber.StatusOK {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("create session status=%d body=%s", response.StatusCode, body)
		}
		var body struct {
			Data struct {
				UserID string `json:"userId"`
			} `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.Data.UserID == "" {
			t.Fatalf("decode session: user=%q error=%v", body.Data.UserID, err)
		}
		return identity{cookie: response.Header.Get(fiber.HeaderSetCookie), userID: body.Data.UserID}
	}
	owner := createSession("聊天甲")
	guest := createSession("聊天乙")
	request := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms", bytes.NewBufferString(`{"joinMode":"open"}`))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	request.Header.Set(fiber.HeaderCookie, owner.cookie)
	response, err := server.App().Test(request)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	defer response.Body.Close()
	var roomBody struct {
		Data struct {
			RoomCode string `json:"roomCode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&roomBody); err != nil || roomBody.Data.RoomCode == "" {
		t.Fatalf("decode room: code=%q error=%v", roomBody.Data.RoomCode, err)
	}
	join := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/join", bytes.NewBufferString(`{"confirmed":true}`))
	join.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	join.Header.Set(fiber.HeaderCookie, guest.cookie)
	joinResponse, err := server.App().Test(join)
	if err != nil || joinResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("join room status=%v response=%v", joinResponse.StatusCode, err)
	}
	_ = joinResponse.Body.Close()
	list := httptest.NewRequest(fiber.MethodGet, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/chat/conversations", nil)
	list.Header.Set(fiber.HeaderCookie, owner.cookie)
	listResponse, err := server.App().Test(list)
	if err != nil || listResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("list conversations status=%v response=%v", listResponse.StatusCode, err)
	}
	_ = listResponse.Body.Close()

	sendBody := `{"recipientUserId":"` + owner.userID + `","clientMessageId":"notify-route","contentText":"跨页面通知"}`
	send := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/chat/messages", bytes.NewBufferString(sendBody))
	send.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	send.Header.Set(fiber.HeaderCookie, guest.cookie)
	sendResponse, err := server.App().Test(send)
	if err != nil || sendResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("send notification message status=%v response=%v", sendResponse.StatusCode, err)
	}
	_ = sendResponse.Body.Close()

	notifications := httptest.NewRequest(fiber.MethodGet, "/api/v1/notifications?limit=1", nil)
	notifications.Header.Set(fiber.HeaderCookie, owner.cookie)
	notificationResponse, err := server.App().Test(notifications)
	if err != nil || notificationResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("list notifications status=%v response=%v", notificationResponse.StatusCode, err)
	}
	defer notificationResponse.Body.Close()
	var notificationBody struct {
		Data struct {
			TotalCount int `json:"totalCount"`
			Items      []struct {
				Type       string `json:"type"`
				PeerUserID string `json:"peerUserId"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(notificationResponse.Body).Decode(&notificationBody); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	if notificationBody.Data.TotalCount != 1 || len(notificationBody.Data.Items) != 1 || notificationBody.Data.Items[0].Type != service.NotificationTypeChatConversation || notificationBody.Data.Items[0].PeerUserID != guest.userID {
		t.Fatalf("notification response = %+v", notificationBody.Data)
	}

	invalid := httptest.NewRequest(fiber.MethodGet, "/api/v1/notifications?limit=0", nil)
	invalid.Header.Set(fiber.HeaderCookie, owner.cookie)
	invalidResponse, err := server.App().Test(invalid)
	if err != nil || invalidResponse.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("invalid notification limit status=%v response=%v", invalidResponse.StatusCode, err)
	}
	_ = invalidResponse.Body.Close()
	unauthorizedResponse, err := server.App().Test(httptest.NewRequest(fiber.MethodGet, "/api/v1/notifications", nil))
	if err != nil || unauthorizedResponse.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("unauthorized notifications status=%v response=%v", unauthorizedResponse.StatusCode, err)
	}
	_ = unauthorizedResponse.Body.Close()
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
				FileID     string `json:"fileId"`
				UploadID   string `json:"uploadId"`
				ChunkSize  int64  `json:"chunkSize"`
				TotalParts int    `json:"totalParts"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(batchResponse.Body).Decode(&batchBody); err != nil || len(batchBody.Data.Files) != 1 {
		t.Fatalf("decode batch response: %+v error=%v", batchBody, err)
	}
	if batchBody.Data.Files[0].UploadID != batchBody.Data.Files[0].FileID || batchBody.Data.Files[0].ChunkSize != 5 || batchBody.Data.Files[0].TotalParts != 1 {
		t.Fatalf("upload plan=%+v", batchBody.Data.Files[0])
	}
	statusRequest := httptest.NewRequest(fiber.MethodGet, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/files/"+batchBody.Data.Files[0].FileID+"/upload", nil)
	statusRequest.Header.Set(fiber.HeaderCookie, cookie)
	statusResponse, err := server.App().Test(statusRequest)
	if err != nil {
		t.Fatal(err)
	}
	var statusBody struct {
		Data struct {
			Status        string `json:"status"`
			ReceivedBytes int64  `json:"receivedBytes"`
			Parts         []any  `json:"parts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(statusResponse.Body).Decode(&statusBody); err != nil {
		_ = statusResponse.Body.Close()
		t.Fatal(err)
	}
	_ = statusResponse.Body.Close()
	if statusResponse.StatusCode != fiber.StatusOK || statusBody.Data.Status != "active" || statusBody.Data.ReceivedBytes != 0 || len(statusBody.Data.Parts) != 0 {
		t.Fatalf("upload status=%d body=%+v", statusResponse.StatusCode, statusBody)
	}
	upload := httptest.NewRequest(fiber.MethodPut, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/files/"+batchBody.Data.Files[0].FileID+"/content", bytes.NewBufferString("hello"))
	upload.Header.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	upload.Header.Set(fiber.HeaderCookie, cookie)
	upload.Header.Set(fiber.HeaderContentRange, "bytes 0-4/5")
	upload.Header.Set("X-Chunk-Number", "0")
	digest := sha256.Sum256([]byte("hello"))
	upload.Header.Set("X-Chunk-SHA256", hex.EncodeToString(digest[:]))
	uploadResponse, err := server.App().Test(upload, 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer uploadResponse.Body.Close()
	if uploadResponse.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(uploadResponse.Body)
		t.Fatalf("upload status=%d body=%s", uploadResponse.StatusCode, body)
	}
	for _, path := range []string{"/api/v1/rooms/" + roomBody.Data.RoomCode + "/files", "/api/v1/rooms/" + roomBody.Data.RoomCode + "/file-events"} {
		request := httptest.NewRequest(fiber.MethodGet, path, nil)
		request.Header.Set(fiber.HeaderCookie, cookie)
		result, err := server.App().Test(request)
		if err != nil {
			t.Fatal(err)
		}
		if result.StatusCode != fiber.StatusOK {
			body, _ := io.ReadAll(result.Body)
			_ = result.Body.Close()
			t.Fatalf("GET %s status=%d body=%s", path, result.StatusCode, body)
		}
		_ = result.Body.Close()
	}
	createDownload := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/files/"+batchBody.Data.Files[0].FileID+"/downloads", nil)
	createDownload.Header.Set(fiber.HeaderCookie, cookie)
	downloadTaskResponse, err := server.App().Test(createDownload)
	if err != nil {
		t.Fatal(err)
	}
	defer downloadTaskResponse.Body.Close()
	var downloadTaskBody struct {
		Data struct {
			DownloadURL string `json:"downloadUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(downloadTaskResponse.Body).Decode(&downloadTaskBody); err != nil || downloadTaskBody.Data.DownloadURL == "" {
		t.Fatalf("decode download task: %+v error=%v", downloadTaskBody, err)
	}
	head := httptest.NewRequest(fiber.MethodHead, downloadTaskBody.Data.DownloadURL, nil)
	head.Header.Set(fiber.HeaderCookie, cookie)
	headResponse, err := server.App().Test(head, 5000)
	if err != nil {
		t.Fatal(err)
	}
	headBody, err := io.ReadAll(headResponse.Body)
	_ = headResponse.Body.Close()
	if err != nil || headResponse.StatusCode != fiber.StatusOK || len(headBody) != 0 || headResponse.Header.Get(fiber.HeaderContentLength) != "5" || headResponse.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("HEAD status=%d body=%q headers=%v error=%v", headResponse.StatusCode, headBody, headResponse.Header, err)
	}

	invalidRange := httptest.NewRequest(fiber.MethodGet, downloadTaskBody.Data.DownloadURL, nil)
	invalidRange.Header.Set(fiber.HeaderCookie, cookie)
	invalidRange.Header.Set(fiber.HeaderRange, "bytes=99-")
	invalidResponse, err := server.App().Test(invalidRange, 5000)
	if err != nil {
		t.Fatal(err)
	}
	var invalidBody struct {
		Code int `json:"code"`
	}
	if err := json.NewDecoder(invalidResponse.Body).Decode(&invalidBody); err != nil {
		_ = invalidResponse.Body.Close()
		t.Fatal(err)
	}
	_ = invalidResponse.Body.Close()
	if invalidResponse.StatusCode != fiber.StatusRequestedRangeNotSatisfiable || invalidBody.Code != 41601 || invalidResponse.Header.Get(fiber.HeaderContentRange) != "bytes */5" {
		t.Fatalf("invalid range status=%d code=%d content-range=%q", invalidResponse.StatusCode, invalidBody.Code, invalidResponse.Header.Get(fiber.HeaderContentRange))
	}

	ranged := httptest.NewRequest(fiber.MethodGet, downloadTaskBody.Data.DownloadURL, nil)
	ranged.Header.Set(fiber.HeaderCookie, cookie)
	ranged.Header.Set(fiber.HeaderRange, "bytes=1-3")
	rangedResponse, err := server.App().Test(ranged, 5000)
	if err != nil {
		t.Fatal(err)
	}
	rangedBody, err := io.ReadAll(rangedResponse.Body)
	_ = rangedResponse.Body.Close()
	if err != nil || rangedResponse.StatusCode != fiber.StatusPartialContent || string(rangedBody) != "ell" || rangedResponse.Header.Get(fiber.HeaderContentRange) != "bytes 1-3/5" || rangedResponse.ContentLength != 3 {
		t.Fatalf("range status=%d body=%q headers=%v error=%v", rangedResponse.StatusCode, rangedBody, rangedResponse.Header, err)
	}

	replay := httptest.NewRequest(fiber.MethodGet, downloadTaskBody.Data.DownloadURL, nil)
	replay.Header.Set(fiber.HeaderCookie, cookie)
	replayResponse, err := server.App().Test(replay, 5000)
	if err != nil {
		t.Fatal(err)
	}
	replayBody, err := io.ReadAll(replayResponse.Body)
	_ = replayResponse.Body.Close()
	if err != nil || replayResponse.StatusCode != fiber.StatusOK || string(replayBody) != "hello" {
		t.Fatalf("replay status=%d body=%q error=%v", replayResponse.StatusCode, replayBody, err)
	}

	createDownloadAgain := httptest.NewRequest(fiber.MethodPost, "/api/v1/rooms/"+roomBody.Data.RoomCode+"/files/"+batchBody.Data.Files[0].FileID+"/downloads", nil)
	createDownloadAgain.Header.Set(fiber.HeaderCookie, cookie)
	againResponse, err := server.App().Test(createDownloadAgain)
	if err != nil {
		t.Fatal(err)
	}
	var againBody struct {
		Data struct {
			DownloadURL string `json:"downloadUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(againResponse.Body).Decode(&againBody); err != nil || againBody.Data.DownloadURL == "" {
		_ = againResponse.Body.Close()
		t.Fatal(err)
	}
	_ = againResponse.Body.Close()
	download := httptest.NewRequest(fiber.MethodGet, againBody.Data.DownloadURL, nil)
	download.Header.Set(fiber.HeaderCookie, cookie)
	downloadResponse, err := server.App().Test(download, 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer downloadResponse.Body.Close()
	downloaded, err := io.ReadAll(downloadResponse.Body)
	if err != nil || string(downloaded) != "hello" || downloadResponse.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(downloadResponse.Header.Get(fiber.HeaderContentDisposition), "attachment") {
		t.Fatalf("download body=%q headers=%v error=%v", downloaded, downloadResponse.Header, err)
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
