package response

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const TranslatorLocalKey = "response_translator"

type Translator func(key string) string

func message(c *fiber.Ctx, key string) string {
	if c != nil {
		if translator, ok := c.Locals(TranslatorLocalKey).(Translator); ok && translator != nil {
			return translator(key)
		}
	}
	return key
}

// Success 成功响应
func Success(c *fiber.Ctx, data interface{}) error {
	return c.JSON(Response{
		Code:    errc.Success,
		Message: message(c, "success"),
		Data:    data,
	})
}

// SuccessWithMessage 成功响应（带自定义消息）
func SuccessWithMessage(c *fiber.Ctx, message string, data interface{}) error {
	return c.JSON(Response{
		Code:    errc.Success,
		Message: responseMessage(c, message),
		Data:    data,
	})
}

// Error 错误响应
func Error(c *fiber.Ctx, code int, message string) error {
	return c.Status(getHTTPStatus(code)).JSON(Response{
		Code:    code,
		Message: responseMessage(c, message),
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(c *fiber.Ctx, code int, message string, data interface{}) error {
	return c.Status(getHTTPStatus(code)).JSON(Response{
		Code:    code,
		Message: responseMessage(c, message),
		Data:    data,
	})
}

// getHTTPStatus 根据业务错误码获取 HTTP 状态码
func getHTTPStatus(code int) int {
	switch {
	case code == errc.Success:
		return fiber.StatusOK
	case code == errc.ErrUnAuthorized || code == errc.ErrAuthExpired || code == errc.ErrTokenPermissionOrTimeout:
		return fiber.StatusUnauthorized
	case code == errc.ErrForbidden || code == errc.ErrInvoiceFileForbidden:
		return fiber.StatusForbidden
	case code == errc.ErrInvoiceFileTooLarge:
		return fiber.StatusRequestEntityTooLarge
	case code == errc.ErrInvoiceFileTypeUnsupported:
		return fiber.StatusUnsupportedMediaType
	case code == errc.ErrInvoiceCandidateInvalid || code == errc.ErrInvoicePreviewInvalid:
		return fiber.StatusUnprocessableEntity
	case code == errc.ErrNotFound || code == errc.ErrUserNotFound:
		return fiber.StatusNotFound
	case code == errc.ErrConflict || code == errc.ErrDuplicate || code == errc.ErrExisted || code == errc.ErrInvoiceFileDuplicate || code == errc.ErrInvoiceBatchStateConflict || code == errc.ErrInvoiceIdentityDuplicate || code == errc.ErrInvoiceRemoveForbidden:
		return fiber.StatusConflict
	case code >= 5000:
		return fiber.StatusInternalServerError
	case code >= 4000:
		return fiber.StatusBadRequest
	default:
		// Default to 200 OK for other business errors (like 3000 Failed)
		// so the frontend can read the JSON body
		return fiber.StatusOK
	}
}

// Created 创建成功响应
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Code:    errc.Success,
		Message: message(c, "created"),
		Data:    data,
	})
}

// NoContent 无内容响应
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Unauthorized 未授权响应
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "error.unauthorized"
	}
	return c.Status(fiber.StatusUnauthorized).JSON(Response{
		Code:    errc.ErrUnAuthorized,
		Message: responseMessage(c, message),
	})
}

// Forbidden 禁止访问响应
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "error.forbidden"
	}
	return c.Status(fiber.StatusForbidden).JSON(Response{
		Code:    errc.ErrForbidden,
		Message: responseMessage(c, message),
	})
}

// NotFound 未找到响应
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "error.not_found"
	}
	return c.Status(fiber.StatusNotFound).JSON(Response{
		Code:    errc.ErrNotFound,
		Message: responseMessage(c, message),
	})
}

// InternalServerError 服务器错误响应
func InternalServerError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "error.internal_server"
	}
	return c.Status(fiber.StatusInternalServerError).JSON(Response{
		Code:    errc.ErrServer,
		Message: responseMessage(c, message),
	})
}

func responseMessage(c *fiber.Ctx, key string) string {
	if key == "" {
		return ""
	}
	return message(c, key)
}

// Failed 失败响应（使用 errx.XError）
func Failed(c *fiber.Ctx, err error) error {
	// 从 errx.XError 中提取 code 和 message
	if err == nil {
		return Success(c, nil)
	}

	code, msg := errx.GetError(err)
	return Error(c, code, msg)
}
