package config

import (
	"time"

	"github.com/spf13/viper"
)

// TraceIDHeader 读写链路 id 的请求/响应头名。
const TraceIDHeader = "X-Request-Id"

// defaultCORSMaxAgeSeconds 预检结果缓存时长。
const defaultCORSMaxAgeSeconds = 1800

// CORS 跨域配置。
type CORS struct {
	// AllowCredentials 是否允许携带凭证(cookie / Authorization)。
	AllowCredentials bool `mapstructure:"allowCredentials"`

	// AllowedOriginPatterns 允许的来源，支持 * 通配，如 https://*.example.com。
	AllowedOriginPatterns []string `mapstructure:"allowedOriginPatterns"`

	// AllowedMethods 允许的请求方法，["*"] 表示回显预检请求的方法。
	AllowedMethods []string `mapstructure:"allowedMethods"`

	// AllowedHeaders 允许的请求头，["*"] 表示回显预检请求的头。
	AllowedHeaders []string `mapstructure:"allowedHeaders"`

	// ExposedHeaders 允许前端 JS 读取的响应头。
	ExposedHeaders []string `mapstructure:"exposedHeaders"`

	// MaxAgeSeconds 预检结果缓存时长(秒)，取值用 MaxAge() 方法。
	MaxAgeSeconds int `mapstructure:"maxAgeSeconds"`
}

// MaxAge 返回预检结果缓存时长。
func (c CORS) MaxAge() time.Duration {
	return time.Duration(c.MaxAgeSeconds) * time.Second
}

// defaultCORS 返回默认配置。
func defaultCORS() CORS {
	return CORS{
		AllowCredentials:      true,
		AllowedOriginPatterns: []string{"*"},
		AllowedMethods:        []string{"*"},
		AllowedHeaders:        []string{"*"},
		ExposedHeaders:        []string{TraceIDHeader},
		MaxAgeSeconds:         defaultCORSMaxAgeSeconds,
	}
}

// setDefaults 把默认值铺给 viper。
func (c CORS) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.cors.allowCredentials", c.AllowCredentials)
	v.SetDefault("middleware.cors.allowedOriginPatterns", c.AllowedOriginPatterns)
	v.SetDefault("middleware.cors.allowedMethods", c.AllowedMethods)
	v.SetDefault("middleware.cors.allowedHeaders", c.AllowedHeaders)
	v.SetDefault("middleware.cors.exposedHeaders", c.ExposedHeaders)
	v.SetDefault("middleware.cors.maxAgeSeconds", c.MaxAgeSeconds)
}

// validate 校验跨域配置。
func (c CORS) validate() error {
	if c.MaxAgeSeconds < 0 {
		return errInvalid("middleware.cors.maxAgeSeconds", "不能为负数")
	}
	if len(c.AllowedOriginPatterns) == 0 {
		return errMissing("middleware.cors.allowedOriginPatterns")
	}
	return nil
}
