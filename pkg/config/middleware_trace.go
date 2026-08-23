package config

import "github.com/spf13/viper"

// TraceID 链路 id 中间件配置。
//
// 原项目没有对照物（全项目零 MDC.put、零 Sleuth，logback-plus.xml 里的
// %tid 是被注释掉的 SkyWalking 残留），这里是 Go 侧自主设计。
//
// 头名常量 TraceIDHeader 定义在 middleware_cors.go —— CORS 的默认
// ExposedHeaders 要引用它，放在被引用方那边省得读者两头找。
type TraceID struct {
	// Header 读写链路 id 的头名，默认 TraceIDHeader。
	Header string `mapstructure:"header"`

	// TrustInbound 是否沿用入站请求头里已有的 id。
	//
	// 默认 true —— 上游 nginx / 网关 / 调用方已经生成过 id 时必须沿用，
	// 否则同一次调用在各进程里拿到不同 id，链路就断了。本项目是
	// 「多模块拆进程 + nginx 负载均衡」，auth 与 system 之间将来若有
	// HTTP 调用，也靠这个头串起来。
	//
	// 反过来，进程直接暴露在公网时应关掉：id 由调用方决定意味着
	// 它可以给一万个请求发同一个 id，把日志检索搅乱。
	TrustInbound bool `mapstructure:"trustInbound"`
}

// defaultTraceID 返回默认配置。
func defaultTraceID() TraceID {
	return TraceID{
		Header:       TraceIDHeader,
		TrustInbound: true,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
func (t TraceID) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.traceId.header", t.Header)
	v.SetDefault("middleware.traceId.trustInbound", t.TrustInbound)
}

// validate 校验链路 id 配置。
//
// Header 留空由 pkg/middleware 回落到 TraceIDHeader，不算错误；
// TrustInbound 是布尔值，无非法取值。
func (t TraceID) validate() error {
	return nil
}
