package config

import (
	"github.com/spf13/viper"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// LocaleHeader 解析语言的请求头名。
//
// **是 content-language，不是 Accept-Language** —— 对齐原项目
// web/core/I18nLocaleResolver.java:24 的 getHeader("content-language")。
//
// 这是个非标准用法：按 RFC 9110，content-language 描述的是**报文自身
// 内容**的语言（「我这个请求体是中文写的」），表达「请把响应给我翻成
// 中文」的标准头是 Accept-Language。但前端发的就是这个头，头名对不上
// 就等于整个 i18n 不生效，所以这里跟着原项目走。
//
// 没有同时兼容 Accept-Language：浏览器会**自动**带上 Accept-Language
// （通常是操作系统语言），一旦回落到它，用户在前端切成英文后，
// 只要某个请求漏发 content-language，就会拿到跟界面语言不一致的文案 ——
// 而这种「偶尔一句中文」的现象极难定位。宁可只认一个显式来源。
const LocaleHeader = "content-language"

// I18n 国际化中间件配置。
type I18n struct {
	// Header 解析语言的请求头名，为空表示用 LocaleHeader。
	//
	// 做成可配是为了将来前端若改用标准的 Accept-Language，
	// 换配置即可，不必改代码。
	Header string `mapstructure:"header"`

	// Default 请求未指定语言、或语言标记不合规时使用的语言。
	//
	// 为空表示用 i18n.DefaultLocale。对应 I18nLocaleResolver 回落
	// Locale.getDefault()，但取的是固定值而非机器区域 —— 详见
	// i18n.DefaultLocale 的说明（多副本部署下机器区域会让同一请求
	// 在不同节点得到不同语言）。
	Default i18n.Locale `mapstructure:"default"`
}

// defaultI18n 返回对齐原项目行为的默认配置。
func defaultI18n() I18n {
	return I18n{
		Header:  LocaleHeader,
		Default: i18n.DefaultLocale,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
func (i I18n) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.i18n.header", i.Header)
	v.SetDefault("middleware.i18n.default", string(i.Default))
}

// validate 校验国际化配置。
//
// 留空表示用 i18n.DefaultLocale，放行；配了就必须能解析 ——
// 一个拼错的语言标记（zh_CN、zh-Hanz）会静默回落到中文，
// 表现为「切了英文没反应」，那种问题没人会想到来查配置文件。
func (i I18n) validate() error {
	if i.Default == "" {
		return nil
	}
	if _, ok := i18n.Parse(string(i.Default)); !ok {
		return errInvalid("middleware.i18n.default", "不是合法的语言标记")
	}
	return nil
}
