package middleware

import (
	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/i18n"
)

// LocaleHeader 解析语言的请求头名。
const LocaleHeader = config.LocaleHeader

// LocaleKey 语言存进 gin.Context 的键名。
const LocaleKey = "locale"

// I18n 国际化中间件，配置取自 config.Get()。
func I18n() gin.HandlerFunc {
	return I18nWithConfig(config.Get().I18n)
}

// I18nWithConfig 国际化中间件。必须注册在 Auth 之前。
func I18nWithConfig(cfg config.I18nConfig) gin.HandlerFunc {
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
		// 解析失败一律用默认语言，不拒绝请求。
		if parsed, ok := i18n.Parse(c.GetHeader(header)); ok {
			loc = parsed
		}

		c.Set(LocaleKey, string(loc))
		c.Request = c.Request.WithContext(i18n.NewContext(c.Request.Context(), loc))

		// 回显实际生效的语言；必须在 c.Next() 之前写。
		c.Writer.Header().Set("Content-Language", string(loc))

		c.Next()
	}
}

// LocaleFrom 从 gin.Context 取本次请求的语言，未设置时返回默认语言。
func LocaleFrom(c *gin.Context) i18n.Locale {
	if c == nil {
		return i18n.DefaultLocale
	}
	if s := c.GetString(LocaleKey); s != "" {
		return i18n.Locale(s)
	}
	return i18n.DefaultLocale
}
