package core

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/leafney/filedock/pkg/i18n"
	"github.com/leafney/filedock/pkg/response"
)

func localizationMiddleware(catalog *i18n.Catalog) fiber.Handler {
	return func(c *fiber.Ctx) error {
		locale := i18n.ParseAcceptLanguage(c.Get(fiber.HeaderAcceptLanguage))
		c.Locals(response.LocaleLocalKey, locale)
		c.Locals(response.TranslatorLocalKey, response.Translator(func(key string, params i18n.Params) string {
			return catalog.Translate(locale, key, params)
		}))
		err := c.Next()
		if strings.HasPrefix(c.GetRespHeader(fiber.HeaderContentType), fiber.MIMEApplicationJSON) {
			setLanguageResponseHeaders(c, locale)
		}
		return err
	}
}

func setLanguageResponseHeaders(c *fiber.Ctx, locale i18n.Locale) {
	c.Set(fiber.HeaderContentLanguage, string(locale))
	c.Vary(fiber.HeaderAcceptLanguage)
}
