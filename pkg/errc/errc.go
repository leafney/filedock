package errc

import (
	"net/http"
	"sort"
)

// Success is the only three-digit code. All business errors use five digits.
const Success = 200

const (
	ErrClient          = 40000
	ErrParams          = 40001
	ErrBindParams      = 40002
	ErrDisplayName     = 40003
	ErrRoomCode        = 40004
	ErrPIN             = 40005
	ErrPINConfirmation = 40006

	ErrUnAuthorized             = 40101
	ErrAuthExpired              = 40102
	ErrTokenInvalid             = 40103
	ErrTokenPermissionOrTimeout = 40104
	ErrRoomPIN                  = 40105

	ErrForbidden      = 40301
	ErrOwnerRequired  = 40302
	ErrMemberRequired = 40303
	ErrKicked         = 40304

	ErrNotFound            = 40401
	ErrUserNotFound        = 40402
	ErrRoomNotFound        = 40403
	ErrJoinRequestNotFound = 40404

	ErrMethodNotAllowed = 40501
	ErrTimeOut          = 40801

	ErrExisted              = 40901
	ErrDuplicate            = 40902
	ErrConflict             = 40903
	ErrNicknameTaken        = 40904
	ErrOwnedRoomLimit       = 40905
	ErrJoinedRoomLimit      = 40906
	ErrRoomFull             = 40907
	ErrRoomNotActive        = 40908
	ErrRoomAlreadyExtended  = 40909
	ErrOwnerCannotLeave     = 40910
	ErrPendingRequestExists = 40911
	ErrPendingRequestLimit  = 40912
	ErrRequestCooldown      = 40913
	ErrIdentityResetBlocked = 40915
	ErrRoomCodeExhausted    = 40916
	ErrJoinModeMismatch     = 40917
	ErrJoinRequestResolved  = 40918

	ErrRateLimited = 42901
	ErrPINPaused   = 42902

	ErrServer    = 50000
	ErrDataBase  = 50001
	ErrCache     = 50002
	ErrJWTKey    = 50003
	ErrLifecycle = 50004
)

// ErrNoLogin is retained as a source-level semantic alias for missing sessions.
const ErrNoLogin = ErrUnAuthorized

type Definition struct {
	Code       int
	MessageKey string
	HTTPStatus int
}

var definitions = map[int]Definition{
	Success:                     {Code: Success, MessageKey: "common.success", HTTPStatus: http.StatusOK},
	ErrClient:                   {Code: ErrClient, MessageKey: "error.client", HTTPStatus: http.StatusBadRequest},
	ErrParams:                   {Code: ErrParams, MessageKey: "error.params", HTTPStatus: http.StatusBadRequest},
	ErrBindParams:               {Code: ErrBindParams, MessageKey: "error.bind_params", HTTPStatus: http.StatusBadRequest},
	ErrDisplayName:              {Code: ErrDisplayName, MessageKey: "error.display_name_invalid", HTTPStatus: http.StatusBadRequest},
	ErrRoomCode:                 {Code: ErrRoomCode, MessageKey: "error.room_code_invalid", HTTPStatus: http.StatusBadRequest},
	ErrPIN:                      {Code: ErrPIN, MessageKey: "error.pin_invalid", HTTPStatus: http.StatusBadRequest},
	ErrPINConfirmation:          {Code: ErrPINConfirmation, MessageKey: "error.pin_confirmation_mismatch", HTTPStatus: http.StatusBadRequest},
	ErrUnAuthorized:             {Code: ErrUnAuthorized, MessageKey: "error.unauthorized", HTTPStatus: http.StatusUnauthorized},
	ErrAuthExpired:              {Code: ErrAuthExpired, MessageKey: "error.auth_expired", HTTPStatus: http.StatusUnauthorized},
	ErrTokenInvalid:             {Code: ErrTokenInvalid, MessageKey: "error.token_invalid", HTTPStatus: http.StatusUnauthorized},
	ErrTokenPermissionOrTimeout: {Code: ErrTokenPermissionOrTimeout, MessageKey: "error.token_invalid_or_expired", HTTPStatus: http.StatusUnauthorized},
	ErrRoomPIN:                  {Code: ErrRoomPIN, MessageKey: "error.room_pin_invalid", HTTPStatus: http.StatusUnauthorized},
	ErrForbidden:                {Code: ErrForbidden, MessageKey: "error.forbidden", HTTPStatus: http.StatusForbidden},
	ErrOwnerRequired:            {Code: ErrOwnerRequired, MessageKey: "error.owner_required", HTTPStatus: http.StatusForbidden},
	ErrMemberRequired:           {Code: ErrMemberRequired, MessageKey: "error.member_required", HTTPStatus: http.StatusForbidden},
	ErrKicked:                   {Code: ErrKicked, MessageKey: "error.kicked", HTTPStatus: http.StatusForbidden},
	ErrNotFound:                 {Code: ErrNotFound, MessageKey: "error.not_found", HTTPStatus: http.StatusNotFound},
	ErrUserNotFound:             {Code: ErrUserNotFound, MessageKey: "error.user_not_found", HTTPStatus: http.StatusNotFound},
	ErrRoomNotFound:             {Code: ErrRoomNotFound, MessageKey: "error.room_not_found", HTTPStatus: http.StatusNotFound},
	ErrJoinRequestNotFound:      {Code: ErrJoinRequestNotFound, MessageKey: "error.join_request_not_found", HTTPStatus: http.StatusNotFound},
	ErrMethodNotAllowed:         {Code: ErrMethodNotAllowed, MessageKey: "error.method_not_allowed", HTTPStatus: http.StatusMethodNotAllowed},
	ErrTimeOut:                  {Code: ErrTimeOut, MessageKey: "error.timeout", HTTPStatus: http.StatusRequestTimeout},
	ErrExisted:                  {Code: ErrExisted, MessageKey: "error.existed", HTTPStatus: http.StatusConflict},
	ErrDuplicate:                {Code: ErrDuplicate, MessageKey: "error.duplicate", HTTPStatus: http.StatusConflict},
	ErrConflict:                 {Code: ErrConflict, MessageKey: "error.conflict", HTTPStatus: http.StatusConflict},
	ErrNicknameTaken:            {Code: ErrNicknameTaken, MessageKey: "error.nickname_taken", HTTPStatus: http.StatusConflict},
	ErrOwnedRoomLimit:           {Code: ErrOwnedRoomLimit, MessageKey: "error.owned_room_limit", HTTPStatus: http.StatusConflict},
	ErrJoinedRoomLimit:          {Code: ErrJoinedRoomLimit, MessageKey: "error.joined_room_limit", HTTPStatus: http.StatusConflict},
	ErrRoomFull:                 {Code: ErrRoomFull, MessageKey: "error.room_full", HTTPStatus: http.StatusConflict},
	ErrRoomNotActive:            {Code: ErrRoomNotActive, MessageKey: "error.room_not_active", HTTPStatus: http.StatusConflict},
	ErrRoomAlreadyExtended:      {Code: ErrRoomAlreadyExtended, MessageKey: "error.room_already_extended", HTTPStatus: http.StatusConflict},
	ErrOwnerCannotLeave:         {Code: ErrOwnerCannotLeave, MessageKey: "error.owner_cannot_leave", HTTPStatus: http.StatusConflict},
	ErrPendingRequestExists:     {Code: ErrPendingRequestExists, MessageKey: "error.pending_request_exists", HTTPStatus: http.StatusConflict},
	ErrPendingRequestLimit:      {Code: ErrPendingRequestLimit, MessageKey: "error.pending_request_limit", HTTPStatus: http.StatusConflict},
	ErrRequestCooldown:          {Code: ErrRequestCooldown, MessageKey: "error.request_cooldown", HTTPStatus: http.StatusConflict},
	ErrIdentityResetBlocked:     {Code: ErrIdentityResetBlocked, MessageKey: "error.identity_reset_blocked", HTTPStatus: http.StatusConflict},
	ErrRoomCodeExhausted:        {Code: ErrRoomCodeExhausted, MessageKey: "error.room_code_exhausted", HTTPStatus: http.StatusConflict},
	ErrJoinModeMismatch:         {Code: ErrJoinModeMismatch, MessageKey: "error.join_mode_mismatch", HTTPStatus: http.StatusConflict},
	ErrJoinRequestResolved:      {Code: ErrJoinRequestResolved, MessageKey: "error.join_request_resolved", HTTPStatus: http.StatusConflict},
	ErrRateLimited:              {Code: ErrRateLimited, MessageKey: "error.rate_limited", HTTPStatus: http.StatusTooManyRequests},
	ErrPINPaused:                {Code: ErrPINPaused, MessageKey: "error.pin_paused", HTTPStatus: http.StatusTooManyRequests},
	ErrServer:                   {Code: ErrServer, MessageKey: "error.internal_server", HTTPStatus: http.StatusInternalServerError},
	ErrDataBase:                 {Code: ErrDataBase, MessageKey: "error.database", HTTPStatus: http.StatusInternalServerError},
	ErrCache:                    {Code: ErrCache, MessageKey: "error.cache", HTTPStatus: http.StatusInternalServerError},
	ErrJWTKey:                   {Code: ErrJWTKey, MessageKey: "error.jwt_key", HTTPStatus: http.StatusInternalServerError},
	ErrLifecycle:                {Code: ErrLifecycle, MessageKey: "error.lifecycle", HTTPStatus: http.StatusInternalServerError},
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
