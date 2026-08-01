package api

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/response"
)

type FileAPI struct {
	biz *biz.FileBiz
}

func NewFileAPI(fileBiz *biz.FileBiz) (*FileAPI, error) {
	if fileBiz == nil {
		return nil, fmt.Errorf("file business is required")
	}
	return &FileAPI{biz: fileBiz}, nil
}

func (a *FileAPI) HandleCreateUploadBatch(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.CreateUploadBatchRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.CreateUploadBatch(principal.UserID, c.Params("code"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Created(c, result)
}

func (a *FileAPI) HandleUploadContent(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	contentLength := int64(c.Context().Request.Header.ContentLength())
	var source io.Reader = c.Context().RequestBodyStream()
	if source == nil {
		source = bytes.NewReader(c.Body())
	}
	if err := a.biz.UploadContent(c.UserContext(), principal.UserID, c.Params("code"), c.Params("fileId"), contentLength, source); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandleCancelUpload(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.CancelUpload(principal.UserID, c.Params("code"), c.Params("fileId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandleList(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, err := a.biz.List(principal.UserID, c.Params("code"), service.FileListQuery{Range: c.Query("range"), Identity: c.Query("identity"), Search: c.Query("search"), Sort: c.Query("sort"), Limit: limit, SharedCursor: c.Query("sharedCursor"), DirectCursor: c.Query("directCursor")})
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleEvents(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, err := a.biz.Events(principal.UserID, c.Params("code"), c.Query("cursor"), limit)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleReusable(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	items, err := a.biz.Reusable(principal.UserID, c.Params("code"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, fiber.Map{"items": items})
}

func (a *FileAPI) HandleAccept(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Accept(principal.UserID, c.Params("code"), c.Params("fileId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandleDecline(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Decline(principal.UserID, c.Params("code"), c.Params("fileId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandleReuse(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.ReusePrivateFilesRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.Reuse(principal.UserID, c.Params("code"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandlePublishShared(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.PublishShared(principal.UserID, c.Params("code"), c.Params("fileId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}
