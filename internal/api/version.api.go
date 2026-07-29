package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
)

type VersionAPI struct {
	biz *biz.VersionBiz
}

func NewVersionAPI(versionBiz *biz.VersionBiz) (*VersionAPI, error) {
	if versionBiz == nil {
		return nil, fmt.Errorf("version business is required")
	}
	return &VersionAPI{biz: versionBiz}, nil
}

func (a *VersionAPI) HandleVersion(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(a.biz.GetVersion())
}
