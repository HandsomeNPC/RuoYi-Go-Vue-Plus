package config

import "github.com/spf13/viper"

// ContentTypeJSON JSON 请求的 content-type 前缀。
const ContentTypeJSON = "application/json"

// defaultMaxBodySize 默认缓存上限 10MB。
const defaultMaxBodySize = 10 << 20

// RepeatableBody 可重复读 body 的配置。
type RepeatableBody struct {
	// ContentTypes 需要缓存的 content-type 前缀（小写），大小写不敏感匹配。
	ContentTypes []string `mapstructure:"contentTypes"`

	// MaxBodySize 允许缓存的最大字节数，超出则拒绝请求。<=0 表示用默认值。
	MaxBodySize int64 `mapstructure:"maxBodySize"`
}

// defaultRepeatableBody 返回默认配置。
func defaultRepeatableBody() RepeatableBody {
	return RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  defaultMaxBodySize,
	}
}

// setDefaults 把默认值铺给 viper。
func (r RepeatableBody) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.repeatableBody.contentTypes", r.ContentTypes)
	v.SetDefault("middleware.repeatableBody.maxBodySize", r.MaxBodySize)
}

// validate 校验可重复读 body 配置。
func (r RepeatableBody) validate() error {
	if r.MaxBodySize < 0 {
		return errInvalid("middleware.repeatableBody.maxBodySize", "不能为负数")
	}
	return nil
}
