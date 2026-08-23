package config

import "github.com/spf13/viper"

// DefaultMaxParamLength 参数日志的最大长度（字符数），对应
// PlusWebInvokeTimeInterceptor.MAX_PARAM_LOG_LENGTH = 4000。
//
// 导出是因为 pkg/middleware 在 MaxParamLength <=0 时要用它兜底。
const DefaultMaxParamLength = 4000

// AccessLog 请求日志配置。
type AccessLog struct {
	// MaxParamLength 参数日志最大字符数，<=0 表示用默认值 4000。
	//
	// 单位是字符（rune）不是字节，对齐 Java 的 substring 语义；
	// 按字节截会把一个中文字符劈成两半，日志里出现乱码。
	MaxParamLength int `mapstructure:"maxParamLength"`

	// SkipPaths 不打日志的路径（精确匹配 URL.Path）。
	//
	// Java 侧没有这个开关（拦截器注册在 /**）。Go 侧加它是给
	// 健康检查、探针这类高频且零信息量的路径用的 —— nginx / k8s
	// 每几秒探一次，不排除掉会把真正有用的日志冲走。
	// 默认为空，即行为与原项目一致。
	SkipPaths []string `mapstructure:"skipPaths"`
}

// defaultAccessLog 返回默认配置，对齐原项目行为。
func defaultAccessLog() AccessLog {
	return AccessLog{
		MaxParamLength: DefaultMaxParamLength,
		SkipPaths:      nil,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
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
