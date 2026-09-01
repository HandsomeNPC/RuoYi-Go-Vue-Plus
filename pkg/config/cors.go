package config

import (
	"time"

	"github.com/spf13/viper"
)

// TraceIDHeader 读写链路 id 的请求/响应头名。
const TraceIDHeader = "X-Request-Id"

// defaultCORSMaxAgeSeconds 预检结果缓存时长。
const defaultCORSMaxAgeSeconds = 1800

// CORSConfig 跨域配置。
type CORSConfig struct {
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
func (c CORSConfig) MaxAge() time.Duration {
	return time.Duration(c.MaxAgeSeconds) * time.Second
}

// DefaultCORS 返回跨域默认配置。
func DefaultCORS() CORSConfig {
	return CORSConfig{
		AllowCredentials:      true,
		AllowedOriginPatterns: []string{"*"},
		AllowedMethods:        []string{"*"},
		AllowedHeaders:        []string{"*"},
		ExposedHeaders:        []string{TraceIDHeader, "Content-Disposition", "download-filename"},
		MaxAgeSeconds:         defaultCORSMaxAgeSeconds,
	}
}

// setDefaults 把默认值铺给 viper。
func (c CORSConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("cors.allowCredentials", c.AllowCredentials)
	v.SetDefault("cors.allowedOriginPatterns", c.AllowedOriginPatterns)
	v.SetDefault("cors.allowedMethods", c.AllowedMethods)
	v.SetDefault("cors.allowedHeaders", c.AllowedHeaders)
	v.SetDefault("cors.exposedHeaders", c.ExposedHeaders)
	v.SetDefault("cors.maxAgeSeconds", c.MaxAgeSeconds)
}

// validate 校验跨域配置。
func (c CORSConfig) validate() error {
	if c.MaxAgeSeconds < 0 {
		return errInvalid("cors.maxAgeSeconds", "不能为负数")
	}
	if len(c.AllowedOriginPatterns) == 0 {
		return errMissing("cors.allowedOriginPatterns")
	}
	return nil
}
