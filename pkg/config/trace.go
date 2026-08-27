package config

import "github.com/spf13/viper"

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

// setDefaults 把默认值铺给 viper。
func (t TraceIDConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("traceId.header", t.Header)
	v.SetDefault("traceId.trustInbound", t.TrustInbound)
}

// validate 校验链路 id 配置。
func (t TraceIDConfig) validate() error {
	return nil
}
