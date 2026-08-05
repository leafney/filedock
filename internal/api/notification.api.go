package api

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
)

type NotificationAPI struct {
	biz *biz.NotificationBiz
}

func (a *NotificationAPI) HandleMarkRead(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.MarkNotificationReadRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.MarkRead(principal.UserID, request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func NewNotificationAPI(notificationBiz *biz.NotificationBiz) (*NotificationAPI, error) {
	if notificationBiz == nil {
		return nil, fmt.Errorf("notification business is required")
	}
	return &NotificationAPI{biz: notificationBiz}, nil
}

func (a *NotificationAPI) HandleList(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	limit := 0
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > service.NotificationMaxPageSize {
			return response.Error(c, errc.ErrParams, nil)
		}
		limit = parsed
	}
	page, err := a.biz.List(principal.UserID, c.Query("cursor"), limit)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, page)
}
