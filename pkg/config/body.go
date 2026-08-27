package config

import "github.com/spf13/viper"

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

// setDefaults 把默认值铺给 viper。
func (r RepeatableBodyConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("repeatableBody.contentTypes", r.ContentTypes)
	v.SetDefault("repeatableBody.maxBodySize", r.MaxBodySize)
}

// validate 校验可重复读 body 配置。
func (r RepeatableBodyConfig) validate() error {
	if r.MaxBodySize < 0 {
		return errInvalid("repeatableBody.maxBodySize", "不能为负数")
	}
	return nil
}
