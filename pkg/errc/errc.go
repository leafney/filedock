package errc

import (
	"net/http"
	"sort"
)

const (
	Success   = 200
	Failed    = 3000
	ErrClient = 4000
	ErrServer = 5000
)

const (
	ErrUnAuthorized             = 4001
	ErrAuthExpired              = 4002
	ErrForbidden                = 4003
	ErrParams                   = 4004
	ErrNoLogin                  = 4005
	ErrBindParams               = 4006
	ErrNotFound                 = 4009
	ErrMethodNotAllowed         = 4010
	ErrTokenPermissionOrTimeout = 4022
	ErrExisted                  = 4023
	ErrDuplicate                = 4027
	ErrUserNotFound             = 4029
	ErrConflict                 = 4031

	ErrTimeOut  = 5001
	ErrDataBase = 5002
	ErrCache    = 5003
)

type Definition struct {
	Code       int
	MessageKey string
	HTTPStatus int
}

var definitions = map[int]Definition{
	Success:                     {Code: Success, MessageKey: "common.success", HTTPStatus: http.StatusOK},
	Failed:                      {Code: Failed, MessageKey: "error.failed", HTTPStatus: http.StatusOK},
	ErrClient:                   {Code: ErrClient, MessageKey: "error.client", HTTPStatus: http.StatusBadRequest},
	ErrUnAuthorized:             {Code: ErrUnAuthorized, MessageKey: "error.unauthorized", HTTPStatus: http.StatusUnauthorized},
	ErrAuthExpired:              {Code: ErrAuthExpired, MessageKey: "error.auth_expired", HTTPStatus: http.StatusUnauthorized},
	ErrForbidden:                {Code: ErrForbidden, MessageKey: "error.forbidden", HTTPStatus: http.StatusForbidden},
	ErrParams:                   {Code: ErrParams, MessageKey: "error.params", HTTPStatus: http.StatusBadRequest},
	ErrNoLogin:                  {Code: ErrNoLogin, MessageKey: "error.no_login", HTTPStatus: http.StatusUnauthorized},
	ErrBindParams:               {Code: ErrBindParams, MessageKey: "error.bind_params", HTTPStatus: http.StatusBadRequest},
	ErrNotFound:                 {Code: ErrNotFound, MessageKey: "error.not_found", HTTPStatus: http.StatusNotFound},
	ErrMethodNotAllowed:         {Code: ErrMethodNotAllowed, MessageKey: "error.method_not_allowed", HTTPStatus: http.StatusMethodNotAllowed},
	ErrTokenPermissionOrTimeout: {Code: ErrTokenPermissionOrTimeout, MessageKey: "error.token_invalid_or_expired", HTTPStatus: http.StatusUnauthorized},
	ErrExisted:                  {Code: ErrExisted, MessageKey: "error.existed", HTTPStatus: http.StatusConflict},
	ErrDuplicate:                {Code: ErrDuplicate, MessageKey: "error.duplicate", HTTPStatus: http.StatusConflict},
	ErrUserNotFound:             {Code: ErrUserNotFound, MessageKey: "error.user_not_found", HTTPStatus: http.StatusNotFound},
	ErrConflict:                 {Code: ErrConflict, MessageKey: "error.conflict", HTTPStatus: http.StatusConflict},
	ErrServer:                   {Code: ErrServer, MessageKey: "error.internal_server", HTTPStatus: http.StatusInternalServerError},
	ErrTimeOut:                  {Code: ErrTimeOut, MessageKey: "error.timeout", HTTPStatus: http.StatusRequestTimeout},
	ErrDataBase:                 {Code: ErrDataBase, MessageKey: "error.database", HTTPStatus: http.StatusInternalServerError},
	ErrCache:                    {Code: ErrCache, MessageKey: "error.cache", HTTPStatus: http.StatusInternalServerError},
}

func Lookup(code int) (Definition, bool) {
	definition, ok := definitions[code]
	return definition, ok
}

func MessageKey(code int) string {
	if definition, ok := Lookup(code); ok {
		return definition.MessageKey
	}
	return definitions[ErrServer].MessageKey
}

func HTTPStatus(code int) int {
	if definition, ok := Lookup(code); ok {
		return definition.HTTPStatus
	}
	return http.StatusInternalServerError
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})
	return result
}
