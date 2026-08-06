package api

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/internal/biz"
	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
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
	result, err := a.biz.AcceptAndDownload(principal.UserID, c.Params("code"), c.Params("fileId"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
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

func (a *FileAPI) HandleCreateDownload(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	result, err := a.biz.CreateDownload(principal.UserID, c.Params("code"), c.Params("fileId"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleDownload(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	descriptor, err := a.biz.InspectDownload(principal.UserID, c.Params("code"), c.Params("taskId"))
	if err != nil {
		return response.Failed(c, err)
	}
	rangeSpec, hasRange, err := service.ParseSingleByteRange(c.Get(fiber.HeaderRange), descriptor.Size)
	if err != nil {
		if hasRange {
			c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes */%d", descriptor.Size))
		}
		return response.Failed(c, errxRangeInvalid(err))
	}
	fileName := strings.ReplaceAll(strings.ReplaceAll(descriptor.FileName, "\r", ""), "\n", "")
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Set(fiber.HeaderContentDisposition, disposition)
	c.Set(fiber.HeaderContentType, descriptor.MIME)
	c.Set("Accept-Ranges", "bytes")
	if hasRange {
		c.Status(fiber.StatusPartialContent)
		c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes %d-%d/%d", rangeSpec.Start, rangeSpec.End, descriptor.Size))
		c.Set(fiber.HeaderContentLength, strconv.FormatInt(rangeSpec.Length(), 10))
	} else {
		c.Set(fiber.HeaderContentLength, strconv.FormatInt(descriptor.Size, 10))
	}
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Method() == fiber.MethodHead {
		return nil
	}
	var requested *service.ByteRange
	if hasRange {
		requested = &rangeSpec
	}
	stream, err := a.biz.BeginDownloadRange(principal.UserID, c.Params("code"), c.Params("taskId"), requested)
	if err != nil {
		return response.Failed(c, err)
	}
	ctx := c.UserContext()
	reader, writer := io.Pipe()
	go func() {
		_ = writer.CloseWithError(stream.WriteTo(ctx, writer))
	}()
	c.Context().SetBodyStream(reader, int(stream.Size()))
	return nil
}

func errxRangeInvalid(err error) error {
	if err == nil {
		return nil
	}
	return errx.New(errc.ErrDownloadRangeInvalid, nil)
}

func (a *FileAPI) HandleTrashList(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, err := a.biz.TrashList(principal.UserID, c.Params("code"), service.FileTrashListQuery{Search: c.Query("search"), Cursor: c.Query("cursor"), Limit: limit})
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleTrash(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.TrashFileRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	result, err := a.biz.Trash(principal.UserID, c.Params("code"), c.Params("fileId"), request)
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleRestore(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	result, err := a.biz.Restore(principal.UserID, c.Params("code"), c.Params("fileId"))
	if err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, result)
}

func (a *FileAPI) HandleApproveRestore(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.ApproveRestore(principal.UserID, c.Params("code"), c.Params("requestId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandleRejectRestore(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	var request dto.RejectFileRestoreRequest
	if err := c.BodyParser(&request); err != nil {
		return response.Error(c, errc.ErrBindParams, nil)
	}
	if err := a.biz.RejectRestore(principal.UserID, c.Params("code"), c.Params("requestId"), request); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}

func (a *FileAPI) HandlePurge(c *fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return response.Error(c, errc.ErrUnAuthorized, nil)
	}
	if err := a.biz.Purge(principal.UserID, c.Params("code"), c.Params("fileId")); err != nil {
		return response.Failed(c, err)
	}
	return response.Success(c, nil)
}
