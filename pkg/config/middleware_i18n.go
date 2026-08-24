package config

import (
	"github.com/spf13/viper"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// LocaleHeader 解析语言的请求头名。
const LocaleHeader = "content-language"

// I18n 国际化中间件配置。
type I18n struct {
	// Header 解析语言的请求头名，为空表示用 LocaleHeader。
	Header string `mapstructure:"header"`

	// Default 请求未指定语言、或语言标记不合规时使用的语言，为空表示用 i18n.DefaultLocale。
	Default i18n.Locale `mapstructure:"default"`
}

// defaultI18n 返回默认配置。
func defaultI18n() I18n {
	return I18n{
		Header:  LocaleHeader,
		Default: i18n.DefaultLocale,
	}
}

// setDefaults 把默认值铺给 viper。
func (i I18n) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.i18n.header", i.Header)
	v.SetDefault("middleware.i18n.default", string(i.Default))
}

// validate 校验国际化配置。
func (i I18n) validate() error {
	if i.Default == "" {
		return nil
	}
	if _, ok := i18n.Parse(string(i.Default)); !ok {
		return errInvalid("middleware.i18n.default", "不是合法的语言标记")
	}
	return nil
}
