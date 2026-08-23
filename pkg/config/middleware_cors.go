package config

import (
	"time"

	"github.com/spf13/viper"
)

// TraceIDHeader 读写链路 id 的请求/响应头名。
//
// 用 X-Request-Id 而非 W3C 的 traceparent：前者是 nginx（$request_id）、
// 各家网关和前端 axios 拦截器的既有约定，接入成本最低。
//
// 定义在本包而非 pkg/middleware，是因为 CORS 的默认 ExposedHeaders 要用它，
// 而 middleware 依赖 config（单向），反过来会成环。middleware 侧留了同名别名。
//
// 放在本文件（而非 middleware_trace.go）是因为 CORS 的默认值要引用它，
// 而 Go 不保证跨文件的初始化顺序可读性 —— 常量无此问题，但读者会去找定义。
const TraceIDHeader = "X-Request-Id"

// defaultCORSMaxAgeSeconds 预检结果缓存时长，对齐 CorsProperties 的 maxAge=1800。
const defaultCORSMaxAgeSeconds = 1800

// CORS 跨域配置，对应 web/config/properties/CorsProperties.java（前缀 web.cors）。
//
// 字段与默认值逐项对齐 Java 侧。注意原项目 application.yml 及 dev/prod profile 里
// 都没有 web.cors 这个 key，所以那边实际生效的是代码默认值 ——
// 本项目把它提到了 yaml 里，默认值与 Java 的代码默认值一致。
type CORS struct {
	// AllowCredentials 是否允许携带凭证(cookie / Authorization)。
	AllowCredentials bool `mapstructure:"allowCredentials"`

	// AllowedOriginPatterns 允许的来源，支持 * 通配，如 https://*.example.com。
	// 对应 Java 的 allowedOriginPatterns 而非 allowedOrigins，
	// 见 pkg/middleware/cors.go 的 matchOrigin 注释。
	AllowedOriginPatterns []string `mapstructure:"allowedOriginPatterns"`

	// AllowedMethods 允许的请求方法，["*"] 表示回显预检请求的方法。
	AllowedMethods []string `mapstructure:"allowedMethods"`

	// AllowedHeaders 允许的请求头，["*"] 表示回显预检请求的头。
	AllowedHeaders []string `mapstructure:"allowedHeaders"`

	// ExposedHeaders 允许前端 JS 读取的响应头。
	//
	// Java 侧没设置这项，跨域下前端只能读到 CORS 安全清单里的那几个头。
	// 默认值有意加了 TraceIDHeader —— 这是相对原项目的**新增行为**，
	// 不加前端就拿不到 traceId，报错时无法和服务端日志对账。
	ExposedHeaders []string `mapstructure:"exposedHeaders"`

	// MaxAgeSeconds 预检结果缓存时长(秒)，对应 maxAge=1800。
	//
	// 存成秒而非 time.Duration：yaml 里写 1800 比写 "30m" 更贴近
	// Java 侧的配置形态，也与 Datasource.ConnMaxLifetime 的写法一致。
	// 取值用 MaxAge() 方法。
	MaxAgeSeconds int `mapstructure:"maxAgeSeconds"`
}

// MaxAge 返回预检结果缓存时长。
func (c CORS) MaxAge() time.Duration {
	return time.Duration(c.MaxAgeSeconds) * time.Second
}

// defaultCORS 返回原项目的代码默认值（CorsProperties 的字段初始值）。
//
// 唯一偏差是 ExposedHeaders：Java 侧为空，这里放了 TraceIDHeader，
// 否则 TraceID 中间件写的响应头跨域时被浏览器挡住，前端读不到。
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

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
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
		// 空列表会拒绝所有跨域请求。这多半是 yaml 里写了 allowedOriginPatterns:
		// 却没给值，而不是真想关掉跨域 —— 真要关就该不注册 CORS 中间件。
		return errMissing("middleware.cors.allowedOriginPatterns")
	}
	return nil
}
