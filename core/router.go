package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/api"
)

func registerRoutes(app *fiber.App, versionAPI *api.VersionAPI) {
	app.Get("/version", versionAPI.HandleVersion)
}
