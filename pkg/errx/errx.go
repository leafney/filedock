package errx

import (
	"errors"
	"fmt"

	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/i18n"
)

type XError struct {
	code   int
	params i18n.Params
	cause  error
}

func New(code int, params i18n.Params) error {
	return &XError{code: normalizedCode(code), params: cloneParams(params)}
}

func Wrap(code int, cause error, params i18n.Params) error {
	return &XError{code: normalizedCode(code), params: cloneParams(params), cause: cause}
}

func (e *XError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.cause != nil {
		return fmt.Sprintf("business error: code=%d: %v", e.code, e.cause)
	}
	return fmt.Sprintf("business error: code=%d", e.code)
}

func (e *XError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func As(err error) (*XError, bool) {
	var target *XError
	if !errors.As(err, &target) {
		return nil, false
	}
	return target, true
}

func Code(err error) int {
	if target, ok := As(err); ok {
		return target.code
	}
	return errc.ErrServer
}

func Params(err error) i18n.Params {
	if target, ok := As(err); ok {
		return cloneParams(target.params)
	}
	return nil
}

func normalizedCode(code int) int {
	if _, ok := errc.Lookup(code); ok {
		return code
	}
	return errc.ErrServer
}

func cloneParams(params i18n.Params) i18n.Params {
	if len(params) == 0 {
		return nil
	}
	cloned := make(i18n.Params, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}
