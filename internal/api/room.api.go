package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
)

type RoomAPI struct {
	biz *biz.RoomBiz
}

func NewRoomAPI(roomBiz *biz.RoomBiz) (*RoomAPI, error) {
	if roomBiz == nil {
		return nil, fmt.Errorf("room business is required")
	}
	return &RoomAPI{biz: roomBiz}, nil
}

func (a *RoomAPI) HandleList(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	items, err := a.biz.List(principal.UserID)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, fiber.Map{"items": items})
}

func (a *RoomAPI) HandleCreate(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.CreateRoomRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.Create(principal.UserID, request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleJoinInfo(c *fiber.Ctx) error {
	principal, _ := Principal(c)
	result, err := a.biz.JoinInfo(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleJoin(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.JoinRoomRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.Join(principal.UserID, c.Params("code"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleSnapshot(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	result, err := a.biz.Snapshot(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleMembers(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	items, err := a.biz.Members(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, fiber.Map{"members": items})
}

func (a *RoomAPI) HandleExtend(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	result, err := a.biz.Extend(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleLeave(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Leave(principal.UserID, c.Params("code")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *RoomAPI) HandleDissolve(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Dissolve(principal.UserID, c.Params("code")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *RoomAPI) HandleKick(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Kick(principal.UserID, c.Params("code"), c.Params("userId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *RoomAPI) HandleQRCode(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if _, err := a.biz.Snapshot(principal.UserID, c.Params("code")); err != nil {
		return response.Failed(c, err)
	}
	content := fmt.Sprintf("%s://%s/rooms/%s", c.Protocol(), c.Hostname(), c.Params("code"))
	svg, err := service.QRCodeSVG(content)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, dto.QRCodeDTO{SVG: svg})
}

func (a *RoomAPI) HandleCreateJoinRequest(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.CreateJoinRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	if !request.Confirmed {
		return response.Error(c, errc.ErrConflict, nil)
	}
	result, err := a.biz.CreateJoinRequest(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleListJoinRequests(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	result, err := a.biz.ListJoinRequests(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *RoomAPI) HandleCancelJoinRequest(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.CancelJoinRequest(principal.UserID, c.Params("code")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *RoomAPI) HandleApproveJoinRequest(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.ApproveJoinRequest(principal.UserID, c.Params("code"), c.Params("requestId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *RoomAPI) HandleRejectJoinRequest(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.RejectJoinRequest(principal.UserID, c.Params("code"), c.Params("requestId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}
