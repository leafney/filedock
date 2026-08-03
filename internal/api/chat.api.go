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

type ChatAPI struct {
	biz *biz.ChatBiz
}

func NewChatAPI(chatBiz *biz.ChatBiz) (*ChatAPI, error) {
	if chatBiz == nil {
		return nil, fmt.Errorf("chat business is required")
	}
	return &ChatAPI{biz: chatBiz}, nil
}

func (a *ChatAPI) HandleListConversations(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	items, err := a.biz.ListConversations(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, fiber.Map{"items": items})
}

func (a *ChatAPI) HandleListMessages(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	before, err := optionalSequence(c.Query("beforeSequence"))
	if err != nil {
		return response.Error(c, errc.ErrParams, nil)
	}
	around, err := optionalSequence(c.Query("aroundSequence"))
	if err != nil {
		return response.Error(c, errc.ErrParams, nil)
	}
	limit, err := optionalInt(c.Query("limit"))
	if err != nil {
		return response.Error(c, errc.ErrParams, nil)
	}
	page, err := a.biz.ListMessages(principal.UserID, c.Params("code"), c.Params("peerUserId"), service.ChatHistoryQuery{BeforeSequence: before, AroundSequence: around, Limit: limit})
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, page)
}

func (a *ChatAPI) HandleSend(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.SendChatMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	item, err := a.biz.Send(principal.UserID, c.Params("code"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, item)
}

func (a *ChatAPI) HandleMarkRead(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.MarkChatReadRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	item, err := a.biz.MarkRead(principal.UserID, c.Params("code"), c.Params("peerUserId"), request.LastReadSequence)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, item)
}

func (a *ChatAPI) HandleRecall(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	item, err := a.biz.Recall(principal.UserID, c.Params("code"), c.Params("messageId"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, item)
}

func (a *ChatAPI) HandleDelete(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.DeleteMessage(principal.UserID, c.Params("code"), c.Params("messageId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *ChatAPI) HandleForward(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.ForwardChatMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	items, err := a.biz.Forward(principal.UserID, c.Params("code"), c.Params("messageId"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, fiber.Map{"items": items})
}

func (a *ChatAPI) HandleSearch(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	before, err := optionalSequence(c.Query("beforeSequence"))
	if err != nil {
		return response.Error(c, errc.ErrParams, nil)
	}
	limit, err := optionalInt(c.Query("limit"))
	if err != nil {
		return response.Error(c, errc.ErrParams, nil)
	}
	term := c.Query("q")
	if term == "" {
		term = c.Query("query")
	}
	page, err := a.biz.Search(principal.UserID, c.Params("code"), c.Params("peerUserId"), service.ChatSearchQuery{Query: term, BeforeSequence: before, Limit: limit})
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, page)
}

func optionalSequence(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return nil, fmt.Errorf("invalid sequence")
	}
	return &parsed, nil
}

func optionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return parsed, nil
}
