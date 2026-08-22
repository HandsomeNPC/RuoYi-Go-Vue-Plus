// Package i18n 国际化词条与渲染，对应原项目 ruoyi-common-core 的
// utils/MessageUtils.java + ruoyi-admin/src/main/resources/i18n/messages*.properties。
//
// Java 侧由 Spring 的 MessageSource（basename: i18n/messages，见
// application.yml:61-63）加载 .properties，当前语言由 LocaleContextHolder
// 这个 ThreadLocal 隐式提供，于是 MessageUtils.message(code, args) 不必传语言。
//
// Go 没有 ThreadLocal，语言**显式随 context.Context 传递**：
// pkg/middleware.I18n 从请求头解析后写进 context，service / handler 调
// Msg(ctx, code, args...) 取。这也是 Go 侧一贯的做法（对齐 trace.go 的
// TraceIDFrom）—— 少一个隐式全局状态，goroutine 里也不会莫名丢语言。
//
// # 词条为什么嵌进源码而不是读 .properties
//
// 本项目是「多模块拆进程、每模块编译成独立 binary」的部署形态。词条编进
// binary 意味着拷一个文件就能跑，不必同步一个 resources 目录；漏拷目录
// 这种故障会等到某条错误路径被触发时才暴露，而那通常是线上。
// 代价是加词条要改 Go 代码重新编译 —— 这些是错误提示文案，本来就跟代码
// 一起走版本，不存在运维单独热改的需求。
//
// # 占位符从 {0} 改成 %v
//
// Java 用 MessageFormat 的位置占位符 {0}/{1}，Go 侧统一改写为 fmt 的 %v，
// 渲染直接走 fmt.Sprintf。与 errs.Newf、enum.LoginType 的模板保持一致 ——
// 那两处已经这么做了，再引入一套 {} 解析只会让同一个仓库里有两种占位符风格。
//
// 三条 length.not.valid / user.username.length.valid / user.password.length.valid
// 里的 {min} {max} **有意保持原样**：那不是 MessageFormat 的位置参数，而是
// Hibernate Validator 的**属性占位符**，由校验注解上的 min/max 属性回填
// （Java 侧也不经 MessageFormat 处理）。本项目的参数校验走 gin binding，
// 阶段 1 接 validator 时再决定怎么回填，现在把它们当字面量原样存着。
package i18n

import (
	"context"
	"fmt"
	"strings"
)

// Locale 语言标记，取值为**归一化后**的 BCP 47 标签（小写、连字符），如 zh-cn / en-us。
//
// 归一化是为了让 map 查表能命中：请求头里可能来 zh_CN、ZH-CN、zh-Hans-CN，
// 都得指向同一份词条。统一在 Parse 里做，本类型的值一律已归一化。
type Locale string

// 已内置词条的语言。
//
// 只有这两个，对齐原项目 i18n 目录下的 messages_zh_CN.properties 与
// messages_en_US.properties。
const (
	LocaleZhCN Locale = "zh-cn"
	LocaleEnUS Locale = "en-us"
)

// DefaultLocale 取不到语言、或语言无对应词条时使用的语言。
//
// 对应 I18nLocaleResolver 取不到请求头时回落的 Locale.getDefault() ——
// 那取的是**JVM 所在机器**的默认区域，本项目部署在中文环境，即 zh-CN。
// 这里写成常量而不是去读操作系统区域：跟着机器走会让同一个请求在不同
// 节点上返回不同语言的文案，而多副本部署下节点是由 nginx 随机挑的。
//
// 另外 messages.properties（无后缀的那份，即 Java 的最终兜底）与
// messages_zh_CN.properties **内容完全一致**（已 diff 确认），
// 所以「兜底」和「中文」在原项目里本来就是同一份词条，这里合并成一个。
const DefaultLocale = LocaleZhCN

// localeMaxLength 允许解析的语言标记最大长度。
//
// 最长的合法标签形如 zh-Hans-CN（10 字符），32 已经宽松到离谱；
// 这个值来自请求头，长度上限是为了不让调用方决定我们拿多长的字符串去查表和打日志。
const localeMaxLength = 32

// catalogs 各语言词条表，键为归一化后的 Locale。
//
// 有意**不提供 Register 之类的注册入口**：那需要一个可变全局 map，
// 而中间件在每个请求里读它 —— 启动后再写就是数据竞争。新增语言直接在
// 本包加一个 messages_xx.go 并往这里加一行，编译期就能发现拼错的键。
var catalogs = map[Locale]map[string]string{
	LocaleZhCN: messagesZhCN,
	LocaleEnUS: messagesEnUS,
}

// langFallback 语言级回落：只给出语言（zh / en）时使用的具体词条。
//
// 显式列出而非遍历 catalogs 找前缀匹配的第一个 —— map 遍历顺序随机，
// 将来同一语言下有两个地区词条（en-US / en-GB）时，遍历版本会让
// `content-language: en` 时而返回美式时而返回英式，且很难复现。
//
// **这是相对 Java 的一处有意偏差**。原项目 i18n 目录下没有
// messages_en.properties，Java 的 ResourceBundle 对 locale=en 的查找链是
// messages_en → （系统默认区域，中文机器上是 messages_zh_CN）→ messages，
// 也就是说原项目里 `content-language: en` 实际会返回**中文**。
// 那是词条文件缺失导致的意外行为而非设计意图（前端真要英文时发的是
// en-US，这条路径平时走不到），照搬只会让「请求英文得到中文」这个
// 结果在 Go 侧也需要一次排查才能解释清楚。
var langFallback = map[string]Locale{
	"zh": LocaleZhCN,
	"en": LocaleEnUS,
}

// localeCtxKey 存进 context.Context 的键。
//
// 私有空结构体，与 trace.go 的 traceIDCtxKey 同一套做法：标准库明确
// 要求自定义 key 类型以避免跨包撞键，且零内存开销。
type localeCtxKey struct{}

// NewContext 返回携带指定语言的子 context。
//
// 由 pkg/middleware.I18n 在每个请求上调用。业务代码一般不直接用它，
// 但在脱离请求的场景（定时任务、消息消费）里需要指定文案语言时可以用。
func NewContext(ctx context.Context, loc Locale) context.Context {
	return context.WithValue(ctx, localeCtxKey{}, loc)
}

// FromContext 取 context 里的语言，取不到返回 DefaultLocale。
//
// 兼容直接传 *gin.Context：gin.Context 实现了 context.Context，
// 其 Value 对非 string 键会回落到 Request.Context()（gin v1.12.0
// context.go:1467-1483 已确认），所以中间件写进 request context 的值
// 在这里取得到。
//
// 取不到返回默认语言而非报错：调用方多半在拼一句提示文案，
// 不该因为少个语言标记就让整个请求失败。
func FromContext(ctx context.Context) Locale {
	if ctx == nil {
		return DefaultLocale
	}
	if loc, ok := ctx.Value(localeCtxKey{}).(Locale); ok && loc != "" {
		return loc
	}
	return DefaultLocale
}

// Parse 归一化并校验语言标记，不合规时 ok 为 false。
//
// 对应 I18nLocaleResolver 里的 `language.replace('_', '-')` +
// Locale.forLanguageTag，另加一层输入校验。
//
// 这个值来自请求头，是**不可信输入**，且会流进日志，所以采用白名单
// （字母数字加连字符、限长）而非过滤坏字符 —— 与 sanitizeTraceID 同样的
// 理由：过滤会把两个不同的输入折叠成同一个，反而制造出对不上的结果。
// 带 CR/LF 的值写进日志就是伪造日志行。
//
// 相对 Java 多做一件事：**按逗号取第一段**。content-language 按 RFC 9110
// 本可以是列表（`en-US, zh-CN`），Java 的 forLanguageTag 遇到这种值会解析
// 成 und（未定语言）从而回落默认语言；这里取第一段，让列表形态也能正常工作。
func Parse(tag string) (Locale, bool) {
	// 列表形态取第一段；单值形态下 Cut 不命中，原样返回。
	tag, _, _ = strings.Cut(tag, ",")

	// 只裁空格与制表符，即 RFC 9110 定义的 OWS（列表逗号两侧允许出现它们）。
	// **有意不用 TrimSpace**：那还会裁掉 \r \n \v \f，于是 "zh-CN\r" 会被
	// 悄悄修好成合法值，而下面的白名单本来是要挡住这类输入的 ——
	// 一个带 CR 的头意味着上游或调用方有问题，值得暴露而不是替它收拾。
	tag = strings.Trim(tag, " \t")

	if tag == "" || len(tag) > localeMaxLength {
		return "", false
	}

	// 下划线归一成连字符，对齐 Java 的 replace('_','-')：
	// 前端和 Java 的 Locale.toString() 都会产出 zh_CN 这种下划线形态。
	tag = strings.ReplaceAll(tag, "_", "-")

	for i := 0; i < len(tag); i++ {
		c := tag[i]
		switch {
		case c >= '0' && c <= '9',
			c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c == '-':
		default:
			return "", false
		}
	}

	// 只做 ASCII 小写，不用 strings.ToLower：语言标签按 BCP 47 限定为 ASCII，
	// 而 ToLower 会对非 ASCII 做 Unicode 折叠（土耳其语 I 之类），
	// 上面的白名单已经把非 ASCII 挡在外面，这里保持行为直白。
	return Locale(asciiLower(tag)), true
}

// asciiLower 把 ASCII 大写字母转小写，其余字节原样保留。
func asciiLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if b == nil {
				// 绝大多数请求头本来就是小写，延迟到真正需要改写时才分配。
				b = []byte(s)
			}
			b[i] = c + ('a' - 'A')
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

// Msg 按 context 里的语言渲染词条，对应 MessageUtils.message(code, args...)。
//
// 词条不存在时**返回 code 本身**，对齐 Java 侧 catch NoSuchMessageException
// 后 return code 的兜底：漏词条时页面上会出现 user.not.exists 这种字符串，
// 丑但能立刻看出缺哪个键，比返回空串（前端显示一片空白，无从下手）好排查。
func Msg(ctx context.Context, code string, args ...any) string {
	return MsgLocale(FromContext(ctx), code, args...)
}

// MsgLocale 按指定语言渲染词条。
//
// 查找顺序：精确匹配 → 语言级回落（en-gb → en → en-us）→ DefaultLocale → 返回 code。
// 对应 Java ResourceBundle 的 locale → language → 默认 locale → 无后缀 兜底链。
func MsgLocale(loc Locale, code string, args ...any) string {
	tmpl, ok := lookup(loc, code)
	if !ok {
		return code
	}
	// 无参数时直接返回模板，不过 Sprintf：模板里的 %v 会被渲染成
	// %!v(MISSING)，而「该带参数却没带」是调用方的 bug，不该由这里
	// 加工成一句更难看的文案。对齐 MessageFormat 在无参时保留 {0} 的行为。
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// lookup 按回落链查词条模板。
func lookup(loc Locale, code string) (string, bool) {
	if code == "" {
		return "", false
	}

	if msg, ok := catalogs[loc][code]; ok {
		return msg, true
	}

	// 语言级回落：zh-hans-cn / en-gb 这类没有专属词条的标签，
	// 退到该语言的默认词条。取第一段（primary language subtag）。
	if lang, _, found := strings.Cut(string(loc), "-"); found {
		if fallback, ok := langFallback[lang]; ok {
			if msg, ok := catalogs[fallback][code]; ok {
				return msg, true
			}
		}
	}

	if msg, ok := catalogs[DefaultLocale][code]; ok {
		return msg, true
	}
	return "", false
}
