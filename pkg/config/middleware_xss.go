package config

import (
	"net/http"

	"github.com/spf13/viper"
)

// defaultXSSSkipMethods XSS 默认跳过清洗的请求方法，对齐 XssFilter.handleExcludeURL
// 里 `HttpMethod.GET.matches(method) || HttpMethod.DELETE.matches(method)` 的判断。
//
// 这两个方法按 REST 语义不携带需要落库的内容，跳过是为了不动查询串 ——
// 搜索关键词里出现 `<` 是正常输入，清洗会把它连同后面的字符一起吃掉。
//
// **但这留下了一个真实的缺口**：带 XSS 载荷的 GET 查询参数不经任何清洗。
// 原项目就是这样，本包对齐；缺口本身由输出侧兜住
// （详见 pkg/middleware/xss.go 的 XSSWithConfig 说明）。
// 做成配置项而非硬编码，是为了让需要收紧的进程能改配置而不必改代码。
var defaultXSSSkipMethods = []string{http.MethodGet, http.MethodDelete}

// XSS 清洗配置，对应 web/config/properties/XssProperties.java（前缀 xss）。
//
// 没有对应 xss.enabled 的字段，原因见 Middleware 的说明。
type XSS struct {
	// ExcludeURLs 跳过清洗的路径，Ant 风格（见 pkg/middleware/path.go）。
	//
	// 对应 xss.excludeUrls（application.yml:193-196），现为
	// /system/notice 与 /warm-flow/save-json —— 富文本公告和流程定义 JSON
	// 需要原样存标签，清洗会直接破坏内容。
	ExcludeURLs []string `mapstructure:"excludeUrls"`

	// SkipMethods 跳过清洗的请求方法，为空表示用 GET/DELETE。
	SkipMethods []string `mapstructure:"skipMethods"`
}

// defaultXSS 返回对齐原项目 yaml 的默认配置。
func defaultXSS() XSS {
	return XSS{
		ExcludeURLs: []string{"/system/notice", "/warm-flow/save-json"},
		SkipMethods: defaultXSSSkipMethods,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
func (x XSS) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.xss.excludeUrls", x.ExcludeURLs)
	v.SetDefault("middleware.xss.skipMethods", x.SkipMethods)
}

// validate 校验 XSS 配置。
//
// 两个字段都是「空列表也合法」的语义（不配排除路径、不配跳过方法都讲得通，
// 后者由 pkg/middleware 回落到 GET/DELETE），没有可拦的非法取值。
// 留一个空实现而不是在汇总处特判，是为了让将来加字段时有地方放。
func (x XSS) validate() error {
	return nil
}
