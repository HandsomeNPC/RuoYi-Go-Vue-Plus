package config

import (
	"net/http"

	"github.com/spf13/viper"
)

// defaultXSSSkipMethods XSS 默认跳过清洗的请求方法。
var defaultXSSSkipMethods = []string{http.MethodGet, http.MethodDelete}

// XSS 清洗配置。
type XSS struct {
	// ExcludeURLs 跳过清洗的路径，Ant 风格。
	ExcludeURLs []string `mapstructure:"excludeUrls"`

	// SkipMethods 跳过清洗的请求方法，为空表示用 GET/DELETE。
	SkipMethods []string `mapstructure:"skipMethods"`
}

// defaultXSS 返回默认配置。
func defaultXSS() XSS {
	return XSS{
		ExcludeURLs: []string{"/system/notice", "/warm-flow/save-json"},
		SkipMethods: defaultXSSSkipMethods,
	}
}

// setDefaults 把默认值铺给 viper。
func (x XSS) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.xss.excludeUrls", x.ExcludeURLs)
	v.SetDefault("middleware.xss.skipMethods", x.SkipMethods)
}

// validate 校验 XSS 配置。
func (x XSS) validate() error {
	return nil
}
