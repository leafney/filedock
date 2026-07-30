package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/api"
)

func registerRoutes(app *fiber.App, versionAPI *api.VersionAPI, sessionAPI *api.SessionAPI) {
	app.Get("/version", versionAPI.HandleVersion)
	app.Get("/api/v1/nicknames/random", sessionAPI.HandleRandomNickname)
	app.Post("/api/v1/sessions", sessionAPI.HandleCreate)
	app.Get("/api/v1/sessions/me", sessionAPI.HandleCurrent)
	app.Put("/api/v1/sessions/me", sessionAPI.HandleUpdate)
	app.Delete("/api/v1/sessions/me", sessionAPI.HandleReset)
}
