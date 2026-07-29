package validator

import (
	"errors"
	"reflect"
	"strings"

	localeen "github.com/go-playground/locales/en"
	localezh "github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	playground "github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/leafney/filedock/pkg/i18n"
)

var (
	validate    *playground.Validate
	translators map[i18n.Locale]ut.Translator
)

func init() {
	validate = playground.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	zhLocale := localezh.New()
	enLocale := localeen.New()
	universal := ut.New(zhLocale, zhLocale, enLocale)
	zhTranslator, _ := universal.GetTranslator("zh")
	enTranslator, _ := universal.GetTranslator("en")
	if err := zhTranslations.RegisterDefaultTranslations(validate, zhTranslator); err != nil {
		panic(err)
	}
	if err := enTranslations.RegisterDefaultTranslations(validate, enTranslator); err != nil {
		panic(err)
	}
	translators = map[i18n.Locale]ut.Translator{
		i18n.LocaleZhCN: zhTranslator,
		i18n.LocaleEn:   enTranslator,
	}
}

func Validate(value interface{}) error {
	return validate.Struct(value)
}

func ValidateVar(value interface{}, tag string) error {
	return validate.Var(value, tag)
}

func FirstMessage(err error, locale i18n.Locale) string {
	if err == nil {
		return ""
	}
	translator, ok := translators[locale]
	if !ok {
		translator = translators[i18n.DefaultLocale]
	}
	var validationErrors playground.ValidationErrors
	if errors.As(err, &validationErrors) && len(validationErrors) > 0 {
		return validationErrors[0].Translate(translator)
	}
	if locale == i18n.LocaleEn {
		return "Invalid request parameters"
	}
	return "请求参数错误"
}
