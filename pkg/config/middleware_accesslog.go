package config

import "github.com/spf13/viper"

// DefaultMaxParamLength 参数日志的最大长度（字符数）。
const DefaultMaxParamLength = 4000

// AccessLog 请求日志配置。
type AccessLog struct {
	// MaxParamLength 参数日志最大字符数，<=0 表示用默认值 4000。
	MaxParamLength int `mapstructure:"maxParamLength"`

	// SkipPaths 不打日志的路径（精确匹配 URL.Path）。
	SkipPaths []string `mapstructure:"skipPaths"`
}

// defaultAccessLog 返回默认配置。
func defaultAccessLog() AccessLog {
	return AccessLog{
		MaxParamLength: DefaultMaxParamLength,
		SkipPaths:      nil,
	}
}

// setDefaults 把默认值铺给 viper。
func (a AccessLog) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.accessLog.maxParamLength", a.MaxParamLength)
	v.SetDefault("middleware.accessLog.skipPaths", a.SkipPaths)
}

// validate 校验请求日志配置。
func (a AccessLog) validate() error {
	if a.MaxParamLength < 0 {
		return errInvalid("middleware.accessLog.maxParamLength", "不能为负数")
	}
	return nil
}
