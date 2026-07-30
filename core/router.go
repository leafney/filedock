package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/api"
)

func registerRoutes(app *fiber.App, versionAPI *api.VersionAPI, sessionAPI *api.SessionAPI, roomAPI *api.RoomAPI) {
	app.Get("/version", versionAPI.HandleVersion)
	app.Get("/api/v1/nicknames/random", sessionAPI.HandleRandomNickname)
	app.Post("/api/v1/sessions", sessionAPI.HandleCreate)
	app.Get("/api/v1/sessions/me", sessionAPI.HandleCurrent)
	app.Put("/api/v1/sessions/me", sessionAPI.HandleUpdate)
	app.Delete("/api/v1/sessions/me", sessionAPI.HandleReset)
	app.Get("/api/v1/rooms", roomAPI.HandleList)
	app.Post("/api/v1/rooms", roomAPI.HandleCreate)
	app.Get("/api/v1/rooms/:code/join-info", roomAPI.HandleJoinInfo)
	app.Post("/api/v1/rooms/:code/join", roomAPI.HandleJoin)
	app.Get("/api/v1/rooms/:code", roomAPI.HandleSnapshot)
	app.Get("/api/v1/rooms/:code/members", roomAPI.HandleMembers)
	app.Post("/api/v1/rooms/:code/extend", roomAPI.HandleExtend)
	app.Delete("/api/v1/rooms/:code", roomAPI.HandleDissolve)
	app.Post("/api/v1/rooms/:code/leave", roomAPI.HandleLeave)
	app.Delete("/api/v1/rooms/:code/members/:userId", roomAPI.HandleKick)
	app.Get("/api/v1/rooms/:code/qrcode", roomAPI.HandleQRCode)
	app.Post("/api/v1/rooms/:code/join-requests", roomAPI.HandleCreateJoinRequest)
	app.Get("/api/v1/rooms/:code/join-requests", roomAPI.HandleListJoinRequests)
	app.Delete("/api/v1/rooms/:code/join-requests/me", roomAPI.HandleCancelJoinRequest)
	app.Post("/api/v1/rooms/:code/join-requests/:requestId/approve", roomAPI.HandleApproveJoinRequest)
	app.Post("/api/v1/rooms/:code/join-requests/:requestId/reject", roomAPI.HandleRejectJoinRequest)
}
