package core

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/api"
	"github.com/leafney/filedock/internal/service"
)

func registerRoutes(app *fiber.App, versionAPI *api.VersionAPI, sessionAPI *api.SessionAPI, roomAPI *api.RoomAPI, fileAPI *api.FileAPI, streamAPI *api.StreamAPI, limiter *service.RateLimiter, chatAPI *api.ChatAPI, notificationAPI *api.NotificationAPI) {
	app.Get("/version", versionAPI.HandleVersion)
	app.Get("/api/v1/nicknames/random", rateLimited(limiter, "nickname_random", 30, time.Minute, sessionAPI.HandleRandomNickname))
	app.Post("/api/v1/sessions", rateLimited(limiter, "session_create", 10, time.Minute, sessionAPI.HandleCreate))
	app.Get("/api/v1/sessions/me", sessionAPI.HandleCurrent)
	app.Put("/api/v1/sessions/me", sessionAPI.HandleUpdate)
	app.Delete("/api/v1/sessions/me", sessionAPI.HandleReset)
	app.Get("/api/v1/notifications", notificationAPI.HandleList)
	app.Post("/api/v1/notifications/read", notificationAPI.HandleMarkRead)
	app.Get("/api/v1/rooms", roomAPI.HandleList)
	app.Post("/api/v1/rooms", rateLimited(limiter, "room_create", 5, time.Hour, roomAPI.HandleCreate))
	app.Get("/api/v1/rooms/:code/join-info", rateLimited(limiter, "room_join_info", 30, time.Minute, roomAPI.HandleJoinInfo))
	app.Post("/api/v1/rooms/:code/join", rateLimited(limiter, "room_join", 20, time.Minute, roomAPI.HandleJoin))
	app.Get("/api/v1/rooms/:code", roomAPI.HandleSnapshot)
	app.Get("/api/v1/rooms/:code/members", roomAPI.HandleMembers)
	app.Post("/api/v1/rooms/:code/extend", roomAPI.HandleExtend)
	app.Delete("/api/v1/rooms/:code", roomAPI.HandleDissolve)
	app.Post("/api/v1/rooms/:code/leave", roomAPI.HandleLeave)
	app.Delete("/api/v1/rooms/:code/members/:userId", roomAPI.HandleKick)
	app.Get("/api/v1/rooms/:code/qrcode", roomAPI.HandleQRCode)
	app.Post("/api/v1/rooms/:code/join-requests", rateLimited(limiter, "room_join_request", 20, time.Minute, roomAPI.HandleCreateJoinRequest))
	app.Get("/api/v1/rooms/:code/join-requests", roomAPI.HandleListJoinRequests)
	app.Delete("/api/v1/rooms/:code/join-requests/me", roomAPI.HandleCancelJoinRequest)
	app.Post("/api/v1/rooms/:code/join-requests/:requestId/approve", roomAPI.HandleApproveJoinRequest)
	app.Post("/api/v1/rooms/:code/join-requests/:requestId/reject", roomAPI.HandleRejectJoinRequest)
	app.Post("/api/v1/rooms/:code/file-upload-batches", fileAPI.HandleCreateUploadBatch)
	app.Put("/api/v1/rooms/:code/files/:fileId/content", fileAPI.HandleUploadContent)
	app.Delete("/api/v1/rooms/:code/files/:fileId/upload", fileAPI.HandleCancelUpload)
	app.Get("/api/v1/rooms/:code/files", fileAPI.HandleList)
	app.Get("/api/v1/rooms/:code/file-events", fileAPI.HandleEvents)
	app.Get("/api/v1/rooms/:code/private-files/reusable", fileAPI.HandleReusable)
	app.Post("/api/v1/rooms/:code/files/:fileId/accept", fileAPI.HandleAccept)
	app.Post("/api/v1/rooms/:code/files/:fileId/decline", fileAPI.HandleDecline)
	app.Post("/api/v1/rooms/:code/private-files/reuse", fileAPI.HandleReuse)
	app.Post("/api/v1/rooms/:code/files/:fileId/publish-shared", fileAPI.HandlePublishShared)
	app.Post("/api/v1/rooms/:code/files/:fileId/downloads", fileAPI.HandleCreateDownload)
	app.Get("/api/v1/rooms/:code/downloads/:taskId", fileAPI.HandleDownload)
	app.Get("/api/v1/rooms/:code/chat/conversations", chatAPI.HandleListConversations)
	app.Get("/api/v1/rooms/:code/chat/conversations/:peerUserId/messages", chatAPI.HandleListMessages)
	app.Post("/api/v1/rooms/:code/chat/messages", chatAPI.HandleSend)
	app.Post("/api/v1/rooms/:code/chat/conversations/:peerUserId/read", chatAPI.HandleMarkRead)
	app.Post("/api/v1/rooms/:code/chat/messages/:messageId/recall", chatAPI.HandleRecall)
	app.Delete("/api/v1/rooms/:code/chat/messages/:messageId", chatAPI.HandleDelete)
	app.Post("/api/v1/rooms/:code/chat/messages/:messageId/forward", chatAPI.HandleForward)
	app.Get("/api/v1/rooms/:code/chat/conversations/:peerUserId/search", chatAPI.HandleSearch)
	app.Get("/api/v1/stream", streamAPI.Handle)
}
