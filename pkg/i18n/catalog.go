package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

const safeFallbackKey = "error.internal_server"

//go:embed locales/*.json
var embeddedLocales embed.FS

type Params map[string]interface{}

type MissingKeyHandler func(locale Locale, key string, err error)

type Option func(*catalogOptions)

type catalogOptions struct {
	missingKeyHandler MissingKeyHandler
}

func WithMissingKeyHandler(handler MissingKeyHandler) Option {
	return func(options *catalogOptions) {
		options.missingKeyHandler = handler
	}
}

type Catalog struct {
	templates         map[Locale]map[string]*template.Template
	missingKeyHandler MissingKeyHandler
}

func NewCatalog(options ...Option) (*Catalog, error) {
	return NewCatalogFromFS(embeddedLocales, options...)
}

func NewCatalogFromFS(localeFS fs.FS, options ...Option) (*Catalog, error) {
	if localeFS == nil {
		return nil, fmt.Errorf("locale filesystem is required")
	}

	opts := catalogOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	catalog := &Catalog{
		templates:         make(map[Locale]map[string]*template.Template, len(supportedLocales)),
		missingKeyHandler: opts.missingKeyHandler,
	}
	var baseKeys map[string]struct{}
	for _, locale := range supportedLocales {
		messages, err := loadMessages(localeFS, locale)
		if err != nil {
			return nil, err
		}
		if baseKeys == nil {
			baseKeys = messageKeys(messages)
		} else if err := validateMessageKeys(locale, baseKeys, messages); err != nil {
			return nil, err
		}

		catalog.templates[locale] = make(map[string]*template.Template, len(messages))
		for key, message := range messages {
			parsed, err := template.New(key).Option("missingkey=error").Parse(message)
			if err != nil {
				return nil, fmt.Errorf("parse %s translation %q: %w", locale, key, err)
			}
			catalog.templates[locale][key] = parsed
		}
	}

	if _, ok := baseKeys[safeFallbackKey]; !ok {
		return nil, fmt.Errorf("safe fallback translation %q is required", safeFallbackKey)
	}
	return catalog, nil
}

func loadMessages(localeFS fs.FS, locale Locale) (map[string]string, error) {
	path := fmt.Sprintf("locales/%s.json", locale)
	data, err := fs.ReadFile(localeFS, path)
	if err != nil {
		return nil, fmt.Errorf("read %s translations: %w", locale, err)
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("decode %s translations: %w", locale, err)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("%s translations are empty", locale)
	}
	for key, message := range messages {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s translations contain an empty key", locale)
		}
		if strings.TrimSpace(message) == "" {
			return nil, fmt.Errorf("%s translation %q is empty", locale, key)
		}
	}
	return messages, nil
}

func messageKeys(messages map[string]string) map[string]struct{} {
	keys := make(map[string]struct{}, len(messages))
	for key := range messages {
		keys[key] = struct{}{}
	}
	return keys
}

func validateMessageKeys(locale Locale, baseKeys map[string]struct{}, messages map[string]string) error {
	for key := range baseKeys {
		if _, ok := messages[key]; !ok {
			return fmt.Errorf("%s translation %q is missing", locale, key)
		}
	}
	for key := range messages {
		if _, ok := baseKeys[key]; !ok {
			return fmt.Errorf("%s translation %q is not defined in %s", locale, key, DefaultLocale)
		}
	}
	return nil
}

func (c *Catalog) Has(locale Locale, key string) bool {
	if c == nil {
		return false
	}
	messages, ok := c.templates[locale]
	if !ok {
		return false
	}
	_, ok = messages[key]
	return ok
}

func (c *Catalog) Translate(locale Locale, key string, params Params) string {
	if c == nil {
		return "服务器繁忙，请稍后重试"
	}
	if normalized, ok := Normalize(string(locale)); ok {
		locale = normalized
	} else {
		locale = DefaultLocale
	}

	message, err := c.execute(locale, key, params)
	if err == nil {
		return message
	}
	c.reportMissing(locale, key, err)

	if locale != DefaultLocale {
		message, fallbackErr := c.execute(DefaultLocale, key, params)
		if fallbackErr == nil {
			return message
		}
		c.reportMissing(DefaultLocale, key, fallbackErr)
	}

	message, fallbackErr := c.execute(DefaultLocale, safeFallbackKey, nil)
	if fallbackErr == nil {
		return message
	}
	c.reportMissing(DefaultLocale, safeFallbackKey, fallbackErr)
	return "服务器繁忙，请稍后重试"
}

func (c *Catalog) execute(locale Locale, key string, params Params) (string, error) {
	messages, ok := c.templates[locale]
	if !ok {
		return "", fmt.Errorf("locale is not loaded")
	}
	messageTemplate, ok := messages[key]
	if !ok {
		return "", fmt.Errorf("translation key is missing")
	}
	var output strings.Builder
	if err := messageTemplate.Execute(&output, params); err != nil {
		return "", fmt.Errorf("execute translation: %w", err)
	}
	return output.String(), nil
}

func (c *Catalog) reportMissing(locale Locale, key string, err error) {
	if c.missingKeyHandler != nil {
		c.missingKeyHandler(locale, key, err)
	}
}
