package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"ruoyi-go-vue-plus/pkg/i18n"
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

// SetDefaults 把默认值铺给 viper。
func (c CORSConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("cors.allowCredentials", c.AllowCredentials)
	v.SetDefault("cors.allowedOriginPatterns", c.AllowedOriginPatterns)
	v.SetDefault("cors.allowedMethods", c.AllowedMethods)
	v.SetDefault("cors.allowedHeaders", c.AllowedHeaders)
	v.SetDefault("cors.exposedHeaders", c.ExposedHeaders)
	v.SetDefault("cors.maxAgeSeconds", c.MaxAgeSeconds)
}

// Validate 校验跨域配置。
func (c CORSConfig) Validate() error {
	if c.MaxAgeSeconds < 0 {
		return fmt.Errorf("cors.maxAgeSeconds 不能为负数")
	}
	if len(c.AllowedOriginPatterns) == 0 {
		return fmt.Errorf("cors.allowedOriginPatterns 未配置")
	}
	return nil
}

// defaultXSSSkipMethods XSS 默认跳过清洗的请求方法。
var defaultXSSSkipMethods = []string{http.MethodGet, http.MethodDelete}

// XSSConfig 清洗配置。
type XSSConfig struct {
	// ExcludeURLs 跳过清洗的路径，Ant 风格。
	ExcludeURLs []string `mapstructure:"excludeUrls"`

	// SkipMethods 跳过清洗的请求方法，为空表示用 GET/DELETE。
	SkipMethods []string `mapstructure:"skipMethods"`
}

// DefaultXSS 返回清洗默认配置。
func DefaultXSS() XSSConfig {
	return XSSConfig{
		ExcludeURLs: []string{"/system/notice", "/warm-flow/save-json"},
		SkipMethods: defaultXSSSkipMethods,
	}
}

// SetDefaults 把默认值铺给 viper。
func (x XSSConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("xss.excludeUrls", x.ExcludeURLs)
	v.SetDefault("xss.skipMethods", x.SkipMethods)
}

// Validate 校验 XSS 配置。
func (x XSSConfig) Validate() error { return nil }

// DefaultMaxParamLength 参数日志的最大长度（字符数）。
const DefaultMaxParamLength = 4000

// AccessLogConfig 请求日志配置。
type AccessLogConfig struct {
	// MaxParamLength 参数日志最大字符数，<=0 表示用默认值 4000。
	MaxParamLength int `mapstructure:"maxParamLength"`

	// SkipPaths 不打日志的路径（精确匹配 URL.Path）。
	SkipPaths []string `mapstructure:"skipPaths"`
}

// DefaultAccessLog 返回请求日志默认配置。
func DefaultAccessLog() AccessLogConfig {
	return AccessLogConfig{
		MaxParamLength: DefaultMaxParamLength,
		SkipPaths:      nil,
	}
}

// SetDefaults 把默认值铺给 viper。
func (a AccessLogConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("accessLog.maxParamLength", a.MaxParamLength)
	v.SetDefault("accessLog.skipPaths", a.SkipPaths)
}

// Validate 校验请求日志配置。
func (a AccessLogConfig) Validate() error {
	if a.MaxParamLength < 0 {
		return fmt.Errorf("accessLog.maxParamLength 不能为负数")
	}
	return nil
}

// TraceIDConfig 链路 id 中间件配置。
type TraceIDConfig struct {
	// Header 读写链路 id 的头名，默认 TraceIDHeader。
	Header string `mapstructure:"header"`

	// TrustInbound 是否沿用入站请求头里已有的 id。
	TrustInbound bool `mapstructure:"trustInbound"`
}

// DefaultTraceID 返回链路 id 默认配置。
func DefaultTraceID() TraceIDConfig {
	return TraceIDConfig{
		Header:       TraceIDHeader,
		TrustInbound: true,
	}
}

// SetDefaults 把默认值铺给 viper。
func (t TraceIDConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("traceId.header", t.Header)
	v.SetDefault("traceId.trustInbound", t.TrustInbound)
}

// Validate 校验链路 id 配置。
func (t TraceIDConfig) Validate() error { return nil }

// ContentTypeJSON JSON 请求的 content-type 前缀。
const ContentTypeJSON = "application/json"

// defaultMaxBodySize 默认缓存上限 10MB。
const defaultMaxBodySize = 10 << 20

// RepeatableBodyConfig 可重复读 body 的配置。
type RepeatableBodyConfig struct {
	// ContentTypes 需要缓存的 content-type 前缀（小写），大小写不敏感匹配。
	ContentTypes []string `mapstructure:"contentTypes"`

	// MaxBodySize 允许缓存的最大字节数，超出则拒绝请求。<=0 表示用默认值。
	MaxBodySize int64 `mapstructure:"maxBodySize"`
}

// DefaultRepeatableBody 返回可重复读 body 默认配置。
func DefaultRepeatableBody() RepeatableBodyConfig {
	return RepeatableBodyConfig{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  defaultMaxBodySize,
	}
}

// SetDefaults 把默认值铺给 viper。
func (r RepeatableBodyConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("repeatableBody.contentTypes", r.ContentTypes)
	v.SetDefault("repeatableBody.maxBodySize", r.MaxBodySize)
}

// Validate 校验可重复读 body 配置。
func (r RepeatableBodyConfig) Validate() error {
	if r.MaxBodySize < 0 {
		return fmt.Errorf("repeatableBody.maxBodySize 不能为负数")
	}
	return nil
}

// LocaleHeader 解析语言的请求头名。
const LocaleHeader = "content-language"

// I18nConfig 国际化中间件配置。
type I18nConfig struct {
	// Header 解析语言的请求头名，为空表示用 LocaleHeader。
	Header string `mapstructure:"header"`

	// Default 请求未指定语言、或语言标记不合规时使用的语言，为空表示用 i18n.DefaultLocale。
	Default i18n.Locale `mapstructure:"default"`
}

// DefaultI18n 返回国际化默认配置。
func DefaultI18n() I18nConfig {
	return I18nConfig{
		Header:  LocaleHeader,
		Default: i18n.DefaultLocale,
	}
}

// SetDefaults 把默认值铺给 viper。
func (i I18nConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("i18n.header", i.Header)
	v.SetDefault("i18n.default", string(i.Default))
}

// Validate 校验国际化配置。
func (i I18nConfig) Validate() error {
	if i.Default == "" {
		return nil
	}
	if _, ok := i18n.Parse(string(i.Default)); !ok {
		return fmt.Errorf("i18n.default 不是合法的语言标记")
	}
	return nil
}
