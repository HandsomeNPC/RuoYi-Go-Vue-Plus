package constant

// 全局通用键常量，主要用于业务无关的 Redis Key 前缀定义。
const (
	// GlobalRedisKey 全局 redis key。
	GlobalRedisKey = "global:"

	// CaptchaCodeKey 验证码，后接 uuid。
	CaptchaCodeKey = GlobalRedisKey + "captcha_codes:"

	// RepeatSubmitKey 防重提交，后接 用户标识 + 请求路径。
	RepeatSubmitKey = GlobalRedisKey + "repeat_submit:"

	// RateLimitKey 接口限流，后接限流维度标识。
	RateLimitKey = GlobalRedisKey + "rate_limit:"

	// SocialAuthCodeKey 三方认证 state，后接 state 值。
	SocialAuthCodeKey = GlobalRedisKey + "social_auth_codes:"

	// OssDefaultConfigKey 当前生效的对象存储配置键，值是 sys_oss_config.config_key。
	// 与 CacheSysOssConfig 缓存组分工：那边存各配置的完整 JSON，这里只记用哪一个。
	OssDefaultConfigKey = GlobalRedisKey + "sys_oss:default_config"
)
