// Package i18n 国际化词条与渲染。
package i18n

import (
	"context"
	"fmt"
	"strings"
)

// Locale 语言标记，取值为归一化后的 BCP 47 标签（小写、连字符），如 zh-cn / en-us。
type Locale string

// 已内置词条的语言。
const (
	LocaleZhCN Locale = "zh-cn"
	LocaleEnUS Locale = "en-us"
)

// DefaultLocale 取不到语言、或语言无对应词条时使用的语言。
const DefaultLocale = LocaleZhCN

// localeMaxLength 允许解析的语言标记最大长度。
const localeMaxLength = 32

// catalogs 各语言词条表，键为归一化后的 Locale。
var catalogs = map[Locale]map[string]string{
	LocaleZhCN: messagesZhCN,
	LocaleEnUS: messagesEnUS,
}

// langFallback 语言级回落：只给出语言（zh / en）时使用的具体词条。
var langFallback = map[string]Locale{
	"zh": LocaleZhCN,
	"en": LocaleEnUS,
}

// localeCtxKey 存进 context.Context 的键。
type localeCtxKey struct{}

// NewContext 返回携带指定语言的子 context。
func NewContext(ctx context.Context, loc Locale) context.Context {
	return context.WithValue(ctx, localeCtxKey{}, loc)
}

// FromContext 取 context 里的语言，取不到返回 DefaultLocale。
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
func Parse(tag string) (Locale, bool) {
	// 列表形态取第一段。
	tag, _, _ = strings.Cut(tag, ",")

	// 只裁空格与制表符（OWS），有意不用 TrimSpace 以挡住 \r \n。
	tag = strings.Trim(tag, " \t")

	if tag == "" || len(tag) > localeMaxLength {
		return "", false
	}

	// 下划线归一成连字符。
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

	return Locale(asciiLower(tag)), true
}

// asciiLower 把 ASCII 大写字母转小写，其余字节原样保留。
func asciiLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if b == nil {
				// 延迟分配。
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

// Msg 按 context 里的语言渲染词条。
func Msg(ctx context.Context, code string, args ...any) string {
	return MsgLocale(FromContext(ctx), code, args...)
}

// MsgLocale 按指定语言渲染词条。
func MsgLocale(loc Locale, code string, args ...any) string {
	tmpl, ok := lookup(loc, code)
	if !ok {
		return code
	}
	// 无参数时直接返回模板，不过 Sprintf。
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

	// 语言级回落：取第一段（primary language subtag）。
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
