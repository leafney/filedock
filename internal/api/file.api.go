package api

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
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
