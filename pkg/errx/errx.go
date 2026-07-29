/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     grape
 * @Date:        2024-11-11 22:30
 * @Description:
 */

package errx

import (
	"errors"
	"fmt"

	"github.com/leafney/filedock/pkg/errc"
)

const defCode = errc.Failed

type XError struct {
	Code int
	Msg  string
}

func (e *XError) Error() string {
	return fmt.Sprintf("error: code = %d desc = %s", e.Code, e.Msg)
}

func ErrorCM(code int, msg string) error {
	return &XError{
		Code: code,
		Msg:  msg,
	}
}

func ErrorCF(code int, format string, a ...interface{}) error {
	return &XError{
		Code: code,
		Msg:  fmt.Sprintf(format, a...),
	}
}

func ErrorCE(code int, err error) error {
	return &XError{
		Code: code,
		Msg:  err.Error(),
	}
}

func ErrorE(err error) error {
	return &XError{
		Code: defCode,
		Msg:  err.Error(),
	}
}

func ErrorM(msg string) error {
	return &XError{
		Code: defCode,
		Msg:  msg,
	}
}

func ErrorMF(format string, a ...interface{}) error {
	return &XError{
		Code: defCode,
		Msg:  fmt.Sprintf(format, a...),
	}
}

func GetError(err error) (int, string) {
	//if s, ok := status.FromError(err); ok {
	//	// err 为 nil 时，返回的 code 为 0
	//	// 如果是默认的 err , code 为 Unknown，需要排除这种情况，改用自己的自定义默认错误码
	//	if s.Code() != codes.Unknown {
	//		// 此处返回的Message 为 `rpc error: code = Unknown desc = xxx` 中的 xxx
	//		return int(s.Code()), errors.New(s.Message())
	//	}
	//}

	var s *XError
	if errors.As(err, &s) {
		return s.Code, s.Msg
	}

	return defCode, err.Error()
}

func GetCode(err error) int {
	//// 如果是能解析的错误，则解析得到 code
	//if s, ok := status.FromError(err); ok {
	//	// err 为 nil 时，返回的 code 为 0
	//	// 如果是默认的 err , code 为 Unknown，需要排除这种情况，改用自己的自定义默认错误码
	//	if s.Code() != codes.Unknown {
	//		return int(s.Code())
	//	}
	//}

	var s *XError
	if errors.As(err, &s) {
		return s.Code
	}
	//// 不能解析的错误，返回默认 code
	return defCode
}

func GetMsg(err error) string {
	//if s, ok := status.FromError(err); ok {
	//	return s.Message()
	//}

	var s *XError
	if errors.As(err, &s) {
		return s.Msg
	}
	return err.Error()
}
