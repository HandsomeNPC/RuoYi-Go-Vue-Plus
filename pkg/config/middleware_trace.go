package config

import "github.com/spf13/viper"

// TraceID 链路 id 中间件配置。
type TraceID struct {
	// Header 读写链路 id 的头名，默认 TraceIDHeader。
	Header string `mapstructure:"header"`

	// TrustInbound 是否沿用入站请求头里已有的 id。
	TrustInbound bool `mapstructure:"trustInbound"`
}

// defaultTraceID 返回默认配置。
func defaultTraceID() TraceID {
	return TraceID{
		Header:       TraceIDHeader,
		TrustInbound: true,
	}
}

// setDefaults 把默认值铺给 viper。
func (t TraceID) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.traceId.header", t.Header)
	v.SetDefault("middleware.traceId.trustInbound", t.TrustInbound)
}

// validate 校验链路 id 配置。
func (t TraceID) validate() error {
	return nil
}
