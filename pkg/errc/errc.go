/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     seine
 * @Date:        2024-07-28 20:46
 * @Description:
 */

package errc

import "strings"

const (
	LangDefault = LangZh
	LangZh      = "zh"
	LangEn      = "en"
)

// 通用
const (
	Success   = 0    // 成功
	Failed    = 3000 // 操作错误
	ErrClient = 4000 // 客户端错误
	ErrServer = 5000 // 服务端错误
)

const (
	ErrNetRequest   = 3001 // 网络请求异常
	ErrVideoExpired = 3002 // 视频已失效
	ErrVideoData    = 3003 // 视频信息获取失败

	//
	ErrUnAuthorized               = 4001 // 鉴权失败，token无效 invalid token 需要通过refresh_token重新获取access_token
	ErrAuthExpired                = 4002 // "授权已过期"
	ErrForbidden                  = 4003 // "权限不足，请重新登录"
	ErrParams                     = 4004 // "请求参数错误"
	ErrNoLogin                    = 4005 // "用户未登录"
	ErrBindParams                 = 4006 // "请求参数绑定错误"
	ErrDupUser                    = 4007 // "用户名已存在"
	ErrWrongNamePwd               = 4008 // "用户名或密码错误"
	ErrNotFound                   = 4009 // 所请求的资源不存在，或不可用
	ErrCaptchaEmpty               = 4010 // 验证码为空
	ErrCaptcha                    = 4011 // 验证码错误
	ErrCaptchaMuchTimes           = 4012 // "今日获取验证码次数过多，请明天再试"
	ErrCaptchaSendFail            = 4013 // "验证码发送失败，请稍后再试"
	ErrCaptchaExpire              = 4014 // "验证码已过期，请重新获取"
	ErrCaptchaLimitTime           = 4015 // "验证码获取间隔不能少于60s"
	ErrWrongEmailPwd              = 4016 // 邮箱或密码错误
	ErrWrongPhonePwd              = 4017 // 手机号或密码错误
	ErrPhoneFormat                = 4018 // 手机号格式错误
	ErrEmailFormat                = 4019 // 邮箱格式错误
	ErrAuthBlackList              = 4020 // 授权Token已被拉黑
	ErrUserBlackList              = 4021 // 该用户已被拉黑
	ErrTokenPermissionOrTimeout   = 4022 // Token无权限或已过期
	ErrExisted                    = 4023 // 资源已存在
	ErrPhoneExist                 = 4024 // 手机号已存在
	ErrEmailExist                 = 4025 // 邮箱已存在
	ErrInternal                   = 4026 // 内部错误
	ErrDuplicate                  = 4027 // 数据已存在
	ErrInviteCode                 = 4028 // 邀请码错误
	ErrUserNotFound               = 4029 // 用户不存在
	ErrUserAlreadyExist           = 4030 // 用户已存在
	ErrConflict                   = 4031 // 当前资源状态冲突
	ErrInvoiceFileInvalid         = 4100
	ErrInvoiceFileTooLarge        = 4101
	ErrInvoiceFileTypeUnsupported = 4102
	ErrInvoiceFileDuplicate       = 4103
	ErrInvoiceBatchStateConflict  = 4104
	ErrInvoiceCandidateInvalid    = 4120
	ErrInvoiceIdentityDuplicate   = 4121
	ErrInvoiceRemoveForbidden     = 4131
	ErrInvoiceFileForbidden       = 4132
	ErrInvoicePreviewInvalid      = 4133

	//
	ErrTimeOut       = 5001 // 请求超时失败
	ErrDataBase      = 5002 // 数据库操作错误
	ErrCache         = 5003 // 缓存操作错误
	ErrCacheNotFound = 5004 // 缓存未找到
	ErrJsonMarshal   = 5005 // json Marshal
	ErrJsonUnMarshal = 5006 // json UnMarshal
	ErrJwtToken      = 5007 // token生成失败

)

var ErrMsgZh = map[int]string{
	Success:             "操作成功",
	Failed:              "操作失败",
	ErrClient:           "客户端请求错误",
	ErrServer:           "服务器繁忙，请稍后重试",
	ErrUnAuthorized:     "鉴权失败",
	ErrAuthExpired:      "授权已过期",
	ErrForbidden:        "禁止访问",
	ErrParams:           "请求参数错误",
	ErrNoLogin:          "用户未登录",
	ErrBindParams:       "请求参数绑定错误",
	ErrDupUser:          "手机号已存在",   // 用户名已存在 / 手机号已存在
	ErrWrongNamePwd:     "用户名或密码错误", // 用户名或密码错误
	ErrWrongEmailPwd:    "邮箱或密码错误",
	ErrWrongPhonePwd:    "手机号或密码错误",
	ErrNotFound:         "所请求的资源不存在，或不可用",
	ErrTimeOut:          "请求超时，请稍后重试",
	ErrCaptcha:          "验证码错误",
	ErrCaptchaEmpty:     "验证码为空",
	ErrCaptchaMuchTimes: "验证码获取次数过多，请明天再试",
	ErrCaptchaSendFail:  "验证码发送失败，请稍后再试",
	ErrCaptchaExpire:    "验证码已过期，请重新获取",
	ErrCaptchaLimitTime: "验证码获取间隔不能少于60s",
	ErrDataBase:         "数据库查询异常",
	ErrCache:            "缓存操作异常",
	ErrCacheNotFound:    "缓存中未查询到",
	ErrPhoneFormat:      "手机号格式错误",
	ErrEmailFormat:      "邮箱格式错误",
	ErrJsonMarshal:      "JsonMarshal",
	ErrJsonUnMarshal:    "JsonUnMarshal",
	ErrNetRequest:       "网络请求异常",
	ErrJwtToken:         "Token生成失败",
	ErrExisted:          "资源已存在",
	ErrConflict:         "当前资源状态冲突",
	// zhui
	ErrVideoExpired:             "视频已失效",
	ErrVideoData:                "视频信息获取失败",
	ErrAuthBlackList:            "授权已被限制",
	ErrUserBlackList:            "触发访问限制",
	ErrTokenPermissionOrTimeout: "Token无权限或已过期",
}
var ErrMsgEn = map[int]string{
	Success:             "Success",
	Failed:              "Operation Failed",
	ErrClient:           "Client Error",
	ErrServer:           "Server is busy, please try again later",
	ErrUnAuthorized:     "鉴权失败",
	ErrAuthExpired:      "授权已过期",
	ErrForbidden:        "禁止访问",
	ErrParams:           "请求参数错误",
	ErrNoLogin:          "用户未登录",
	ErrBindParams:       "请求参数绑定错误",
	ErrDupUser:          "手机号已存在",   // 用户名已存在 / 手机号已存在
	ErrWrongNamePwd:     "用户名或密码错误", // 用户名或密码错误
	ErrWrongEmailPwd:    "邮箱或密码错误",
	ErrWrongPhonePwd:    "手机号或密码错误",
	ErrNotFound:         "所请求的资源不存在，或不可用",
	ErrTimeOut:          "请求超时，请稍后重试",
	ErrCaptcha:          "验证码错误",
	ErrCaptchaEmpty:     "验证码为空",
	ErrCaptchaMuchTimes: "验证码获取次数过多，请明天再试",
	ErrCaptchaSendFail:  "验证码发送失败，请稍后再试",
	ErrCaptchaExpire:    "验证码已过期，请重新获取",
	ErrCaptchaLimitTime: "验证码获取间隔不能少于60s",
	ErrDataBase:         "数据库查询异常",
	ErrCache:            "缓存操作异常",
	ErrCacheNotFound:    "缓存中未查询到",
	ErrPhoneFormat:      "手机号格式错误",
	ErrEmailFormat:      "邮箱格式错误",
	ErrJsonMarshal:      "JsonMarshal",
	ErrJsonUnMarshal:    "JsonUnMarshal",
	ErrNetRequest:       "网络请求异常",
	ErrJwtToken:         "Token生成失败",
	ErrExisted:          "资源已存在",
	ErrConflict:         "Resource state conflict",
	// zhui
	ErrVideoExpired:             "视频已失效",
	ErrVideoData:                "视频信息获取失败",
	ErrAuthBlackList:            "授权已被限制",
	ErrUserBlackList:            "触发访问限制",
	ErrTokenPermissionOrTimeout: "Token无权限或已过期",
}

// 根据统一错误码获取相应的错误提示信息
func GetErrMsg(code int, lang string) string {
	var errList = ErrMsgEn
	switch lang {
	case LangZh:
		errList = ErrMsgZh
	case LangEn:
		errList = ErrMsgEn
	default:
		// 判断默认语言
		if strings.EqualFold(LangDefault, LangZh) {
			errList = ErrMsgZh
		} else {
			errList = ErrMsgEn
		}
	}

	msg, ok := errList[code]
	if ok {
		return msg
	}
	return errList[ErrServer]
}
