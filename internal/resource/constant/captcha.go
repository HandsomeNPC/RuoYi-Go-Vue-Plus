package constant

import (
	"regexp"
	"time"
)

// 验证码的 Redis 键与有效期。
//
// resource 是验证码主产出方（短信/邮箱），auth 写图形验证码并在登录时校验，
// 故键约定归此处、由 auth 反向引用。"global:" 前缀沿用全局 redis key 约定。
const (
	// CaptchaCodeKey 验证码，后接 uuid / 手机号 / 邮箱。
	CaptchaCodeKey = "global:captcha_codes:"

	// CaptchaExpirationMinutes 验证码有效期（分钟），短信模板里也按它填文案。
	CaptchaExpirationMinutes = 2
)

// CaptchaExpiration 验证码有效期。
const CaptchaExpiration = CaptchaExpirationMinutes * time.Minute

// 验证码接口的入参校验。
const (
	// RegexMobile 中国大陆手机号。
	RegexMobile = `^(?:0|86|\+86)?1[3-9]\d{9}$`

	// RegexEmail 邮箱地址，对照 hutool Validator.isEmail 所用的模式。
	// 本地部分放行常见的点/加号/连字符等，域名部分要求至少两级且顶级域为字母。
	RegexEmail = `^[\w!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[\w!#$%&'*+/=?^_` + "`" +
		`{|}~-]+)*@(?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}$`
)

var (
	PatternMobile = regexp.MustCompile(RegexMobile)
	PatternEmail  = regexp.MustCompile(RegexEmail)
)
