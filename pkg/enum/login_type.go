package enum

import (
	"context"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// LoginType 登录方式。
type LoginType struct {
	Code string

	// RetryCountKey 未达上限时的提示词条键，有一个占位符：已错误次数。
	RetryCountKey string
	// RetryExceedKey 达到上限时的提示词条键，有两个占位符：最大次数、锁定分钟数。
	RetryExceedKey string
}

// 登录方式枚举实例。Social / Xcx 不做重试计数，词条键为空。
var (
	LoginTypePassword = LoginType{
		Code:           "password",
		RetryCountKey:  "user.password.retry.limit.count",
		RetryExceedKey: "user.password.retry.limit.exceed",
	}
	LoginTypeSms = LoginType{
		Code:           "sms",
		RetryCountKey:  "sms.code.retry.limit.count",
		RetryExceedKey: "sms.code.retry.limit.exceed",
	}
	LoginTypeEmail = LoginType{
		Code:           "email",
		RetryCountKey:  "email.code.retry.limit.count",
		RetryExceedKey: "email.code.retry.limit.exceed",
	}
	LoginTypeSocial = LoginType{
		Code: "social",
	}
	LoginTypeXcx = LoginType{
		Code: "xcx",
	}
)

var loginTypes = []LoginType{
	LoginTypePassword, LoginTypeSms, LoginTypeEmail, LoginTypeSocial, LoginTypeXcx,
}

// LoginTypes 返回全部登录方式的副本。
func LoginTypes() []LoginType {
	return append([]LoginType(nil), loginTypes...)
}

// ParseLoginType 按 Code 查找登录方式，未匹配时 ok 为 false。
func ParseLoginType(code string) (LoginType, bool) {
	for _, t := range loginTypes {
		if t.Code == code {
			return t, true
		}
	}
	return LoginType{}, false
}

// RetryCountMsg 渲染「已错误 errCount 次」提示，不做重试计数时返回空串。
func (t LoginType) RetryCountMsg(ctx context.Context, errCount int) string {
	if t.RetryCountKey == "" {
		return ""
	}
	return i18n.Msg(ctx, t.RetryCountKey, errCount)
}

// RetryExceedMsg 渲染「错误次数超限，账户锁定」提示，不做重试计数时返回空串。
func (t LoginType) RetryExceedMsg(ctx context.Context, maxRetry int, lockMinutes int) string {
	if t.RetryExceedKey == "" {
		return ""
	}
	return i18n.Msg(ctx, t.RetryExceedKey, maxRetry, lockMinutes)
}
