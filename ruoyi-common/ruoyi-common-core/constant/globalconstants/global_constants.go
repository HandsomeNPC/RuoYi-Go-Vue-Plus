// Package globalconstants 全局通用键常量，主要用于业务无关的 Redis Key 前缀定义。
//
// @author Lion Li
package globalconstants

const (
	// GlobalRedisKey 全局 redis key (业务无关的key)
	GlobalRedisKey = "global:"

	// CaptchaCodeKey 验证码 redis key
	CaptchaCodeKey = GlobalRedisKey + "captcha_codes:"

	// RepeatSubmitKey 防重提交 redis key
	RepeatSubmitKey = GlobalRedisKey + "repeat_submit:"

	// RateLimitKey 限流 redis key
	RateLimitKey = GlobalRedisKey + "rate_limit:"

	// SocialAuthCodeKey 三方认证 redis key
	SocialAuthCodeKey = GlobalRedisKey + "social_auth_codes:"
)
