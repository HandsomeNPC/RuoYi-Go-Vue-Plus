package syslogin

import "fmt"

// LoginType 登录方式。登录失败重试计数的提示文案按方式区分，故把模板挂在枚举上。
type LoginType struct {
	Code string

	// RetryCountTmpl 未达上限时的提示模板，有一个占位符：已错误次数。为空表示不做重试计数。
	RetryCountTmpl string
	// RetryExceedTmpl 达到上限时的提示模板，有两个占位符：最大次数、锁定分钟数。为空表示不做重试计数。
	RetryExceedTmpl string
}

// 登录方式枚举实例。Social / Xcx 不做重试计数，模板为空。
var (
	LoginTypePassword = LoginType{
		Code:            "password",
		RetryCountTmpl:  "密码输入错误%v次",
		RetryExceedTmpl: "密码输入错误%v次，账户锁定%v分钟",
	}
	LoginTypeSms = LoginType{
		Code:            "sms",
		RetryCountTmpl:  "短信验证码输入错误%v次",
		RetryExceedTmpl: "短信验证码输入错误%v次，账户锁定%v分钟",
	}
	LoginTypeEmail = LoginType{
		Code:            "email",
		RetryCountTmpl:  "邮箱验证码输入错误%v次",
		RetryExceedTmpl: "邮箱验证码输入错误%v次，账户锁定%v分钟",
	}
	LoginTypeSocial = LoginType{
		Code: "social",
	}
	LoginTypeXcx = LoginType{
		Code: "xcx",
	}
)

// RetryCountMsg 渲染「已错误 errCount 次」提示，不做重试计数时返回空串。
func (t LoginType) RetryCountMsg(errCount int) string {
	if t.RetryCountTmpl == "" {
		return ""
	}
	return fmt.Sprintf(t.RetryCountTmpl, errCount)
}

// RetryExceedMsg 渲染「错误次数超限，账户锁定」提示，不做重试计数时返回空串。
func (t LoginType) RetryExceedMsg(maxRetry, lockMinutes int) string {
	if t.RetryExceedTmpl == "" {
		return ""
	}
	return fmt.Sprintf(t.RetryExceedTmpl, maxRetry, lockMinutes)
}
