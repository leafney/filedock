package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate *validator.Validate
	trans    ut.Translator
)

func init() {
	validate = validator.New()

	// 注册一个函数，获取结构体标签中的 json 名字
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// 注册中文翻译器
	zhTrans := zh.New()
	uni := ut.New(zhTrans, zhTrans)
	trans, _ = uni.GetTranslator("zh")

	// 注册翻译
	_ = zh_translations.RegisterDefaultTranslations(validate, trans)
}

// Validate 验证结构体
func Validate(s interface{}) error {
	err := validate.Struct(s)
	if err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			var errMsgs []string
			for _, e := range errs {
				errMsgs = append(errMsgs, e.Translate(trans))
			}
			return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
		}
		return err
	}
	return nil
}

// ValidateVar 验证变量
func ValidateVar(field interface{}, tag string) error {
	err := validate.Var(field, tag)
	if err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			var errMsgs []string
			for _, e := range errs {
				errMsgs = append(errMsgs, e.Translate(trans))
			}
			return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
		}
		return err
	}
	return nil
}
