package constant

/**
 * 全局通用键常量，主要用于业务无关的 Redis Key 前缀定义。
 *
 * @author Lion Li
 */
const (
	// GlobalRedisKey 全局 redis key (业务无关的key)
	GlobalRedisKey = "global:"

	// CaptchaCodeKey 验证码，后接 uuid。
	CaptchaCodeKey = GlobalRedisKey + "captcha_codes:"

	// RepeatSubmitKey 防重提交，后接 用户标识 + 请求路径。
	RepeatSubmitKey = GlobalRedisKey + "repeat_submit:"

	// RateLimitKey 接口限流，后接限流维度标识。
	RateLimitKey = GlobalRedisKey + "rate_limit:"

	// SocialAuthCodeKey 三方认证 state，后接 state 值。
	SocialAuthCodeKey = GlobalRedisKey + "social_auth_codes:"
)
