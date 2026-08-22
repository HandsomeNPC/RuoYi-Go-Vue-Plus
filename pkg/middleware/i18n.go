package middleware

import (
	"github.com/gin-gonic/gin"

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

// LocaleKey 语言存进 gin.Context 的键名。
//
// 与 TraceIDKey 同样用 camelCase 对齐前端与 Java 侧字段风格。
// handler 可用 c.GetString(LocaleKey) 直接取；service / repository
// 层从 context.Context 取，走 i18n.FromContext。
const LocaleKey = "locale"

// I18nConfig 国际化中间件配置。
type I18nConfig struct {
	// Header 解析语言的请求头名，为空表示用 LocaleHeader。
	//
	// 做成可配是为了将来前端若改用标准的 Accept-Language，
	// 换配置即可，不必改这个文件。
	Header string

	// Default 请求未指定语言、或语言标记不合规时使用的语言。
	//
	// 为空表示用 i18n.DefaultLocale。对应 I18nLocaleResolver 回落
	// Locale.getDefault()，但取的是固定值而非机器区域 —— 详见
	// i18n.DefaultLocale 的说明（多副本部署下机器区域会让同一请求
	// 在不同节点得到不同语言）。
	Default i18n.Locale
}

// DefaultI18nConfig 返回对齐原项目行为的默认配置。
func DefaultI18nConfig() I18nConfig {
	return I18nConfig{
		Header:  LocaleHeader,
		Default: i18n.DefaultLocale,
	}
}

// I18n 国际化中间件，用默认配置。
func I18n() gin.HandlerFunc {
	return I18nWithConfig(DefaultI18nConfig())
}

// I18nWithConfig 国际化中间件，对应原项目
// web/config/I18nConfig.java + web/core/I18nLocaleResolver.java。
//
// 做的事只有一件：从 content-language 请求头解析出本次请求的语言，
// 存进 gin.Context 与 request 的 context.Context，供
// i18n.Msg(ctx, code, args...) 取用（等价于 Java 的
// MessageUtils.message —— 那边语言由 LocaleContextHolder 这个
// ThreadLocal 隐式提供，Go 侧显式随 context 传）。
//
// # 注册位置
//
// 按 README 的顺序表挂在 XSS 之后、Auth 之前：
//
//	Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n → Auth
//
// **必须在 Auth 之前**：阶段 1 的鉴权中间件会返回「客户端ID与Token不匹配」
// 这类文案，那些文案要走词条，就得先有语言。这是本中间件唯一的顺序约束 ——
// 它不读 body、不改请求，与前面几环没有耦合。
//
// 位置靠后不影响前面的中间件：Recover 与 AccessLog 的输出是**日志**，
// 日志面向运维、恒为中文，本来就不该跟着请求语言变（否则同一个错误在
// 日志里有两种文案，检索时得搜两遍）。
//
// # 相对 Java 的偏差
//
// 与原项目最大的不同是 setLocale 没有对应物。Java 的 LocaleResolver 接口
// 强制实现它，原项目给了个空实现（服务端不主动切语言），Go 侧没有这个
// 接口约束，直接不提供 —— 少一个「存在但什么都不做」的方法。
//
// 另外多做两件 Java 没做的事：
//
//   - **校验语言标记**。Java 的 forLanguageTag 对非法输入静默返回 und
//     （未定语言）从而回落默认值，不算漏洞；Go 侧显式走 i18n.Parse 的
//     白名单，因为这个值会进日志，带 CR/LF 的输入能伪造日志行。
//   - **回显 Content-Language 响应头**。见下方说明。
func I18nWithConfig(cfg I18nConfig) gin.HandlerFunc {
	header := cfg.Header
	if header == "" {
		header = LocaleHeader
	}
	def := cfg.Default
	if def == "" {
		def = i18n.DefaultLocale
	}

	return func(c *gin.Context) {
		loc := def
		// 解析失败（空头、超长、含非法字符）一律用默认语言，不拒绝请求：
		// 语言只影响提示文案的呈现，为一个畸形的头把整个业务请求打回去
		// 不成比例。对齐 Java 侧 forLanguageTag 拿到 und 也照常处理。
		if parsed, ok := i18n.Parse(c.GetHeader(header)); ok {
			loc = parsed
		}

		c.Set(LocaleKey, string(loc))
		c.Request = c.Request.WithContext(i18n.NewContext(c.Request.Context(), loc))

		// 回显本次实际生效的语言，这是**相对 Java 的有意新增**（原项目不回）。
		// 客户端发 zh-Hans-CN 而服务端按 zh-CN 出文案时，只看响应体是看不出
		// 这层归一化的；出现「明明发了 en 却收到中文」时，这个头能一眼定位到
		// 是语言协商的结果而不是词条缺失。
		//
		// 与 trace.go 同一条纪律：**必须在 c.Next() 之前写**。body 一开始
		// 输出，header 就已经发出去了，事后 Set 会静默失效。
		//
		// 用规范大小写的 Content-Language（响应头按 RFC 9110 是这个拼写），
		// 请求侧则沿用原项目的全小写 —— Go 的 http.Header 读取时不区分大小写，
		// 但写出去的字面量按规范来。
		c.Writer.Header().Set("Content-Language", string(loc))

		c.Next()
	}
}

// LocaleFrom 从 gin.Context 取本次请求的语言，未设置时返回 i18n.DefaultLocale。
//
// 给 handler 用的便捷函数。service / repository 层拿到的是
// context.Context，应直接用 i18n.FromContext —— 那条路径不必 import gin。
//
// 两者取的是同一个值：本中间件同时写了 gin.Context 和 request context，
// 且 gin.Context 的 Value 对非 string 键会回落到 Request.Context()。
func LocaleFrom(c *gin.Context) i18n.Locale {
	if c == nil {
		return i18n.DefaultLocale
	}
	if s := c.GetString(LocaleKey); s != "" {
		return i18n.Locale(s)
	}
	return i18n.DefaultLocale
}
