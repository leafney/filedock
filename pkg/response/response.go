package response

import (
	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/i18n"
	validation "github.com/leafney/filedock/pkg/validator"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	TranslatorLocalKey = "response_translator"
	LocaleLocalKey     = "response_locale"
)

type Translator func(key string, params i18n.Params) string

func Locale(c *fiber.Ctx) i18n.Locale {
	if c != nil {
		if locale, ok := c.Locals(LocaleLocalKey).(i18n.Locale); ok {
			return locale
		}
	}
	return i18n.DefaultLocale
}

func message(c *fiber.Ctx, key string, params i18n.Params) string {
	if c != nil {
		if translator, ok := c.Locals(TranslatorLocalKey).(Translator); ok && translator != nil {
			return translator(key, params)
		}
	}
	if key == "common.success" {
		return "操作成功"
	}
	if key == "common.created" {
		return "创建成功"
	}
	return "服务器繁忙，请稍后重试"
}

func Success(c *fiber.Ctx, data interface{}) error {
	return write(c, errc.Success, message(c, errc.MessageKey(errc.Success), nil), data)
}

func SuccessWithKey(c *fiber.Ctx, key string, params i18n.Params, data interface{}) error {
	return write(c, errc.Success, message(c, key, params), data)
}

func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Code:    errc.Success,
		Message: message(c, "common.created", nil),
		Data:    data,
	})
}

func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

func Error(c *fiber.Ctx, code int, params i18n.Params) error {
	return write(c, code, message(c, errc.MessageKey(code), params), nil)
}

func ErrorWithData(c *fiber.Ctx, code int, params i18n.Params, data interface{}) error {
	return write(c, code, message(c, errc.MessageKey(code), params), data)
}

func ValidationError(c *fiber.Ctx, err error) error {
	return write(c, errc.ErrParams, validation.FirstMessage(err, Locale(c)), nil)
}

func Unauthorized(c *fiber.Ctx) error {
	return Error(c, errc.ErrUnAuthorized, nil)
}

func Forbidden(c *fiber.Ctx) error {
	return Error(c, errc.ErrForbidden, nil)
}

func NotFound(c *fiber.Ctx) error {
	return Error(c, errc.ErrNotFound, nil)
}

func InternalServerError(c *fiber.Ctx) error {
	return Error(c, errc.ErrServer, nil)
}

func Failed(c *fiber.Ctx, err error) error {
	if err == nil {
		return Success(c, nil)
	}
	return Error(c, errx.Code(err), errx.Params(err))
}

func write(c *fiber.Ctx, code int, translatedMessage string, data interface{}) error {
	if _, ok := errc.Lookup(code); !ok {
		code = errc.ErrServer
		translatedMessage = message(c, errc.MessageKey(code), nil)
	}
	return c.Status(errc.HTTPStatus(code)).JSON(Response{
		Code:    code,
		Message: translatedMessage,
		Data:    data,
	})
}
