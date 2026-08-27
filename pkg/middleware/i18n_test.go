package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// newI18nEngine 构造只挂 I18n 的引擎，回写三处看到的语言。
func newI18nEngine(cfg config.I18nConfig) *gin.Engine {
	r := gin.New()
	r.Use(I18nWithConfig(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"fromGin": c.GetString(LocaleKey),
			"fromCtx": string(i18n.FromContext(c.Request.Context())),
			"helper":  string(LocaleFrom(c)),
			"msg":     i18n.Msg(c.Request.Context(), "user.logout.success"),
		})
	})
	return r
}

// i18nGet 发一次请求，可选带语言头。
func i18nGet(r *gin.Engine, header, lang string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if lang != "" {
		req.Header.Set(header, lang)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestI18nPropagatesToAllContexts 语言必须同时到达三处上下文，且文案按该语言渲染。
func TestI18nPropagatesToAllContexts(t *testing.T) {
	cases := []struct {
		name    string
		lang    string
		wantLoc string
		wantMsg string
	}{
		{"中文", "zh-CN", "zh-cn", "退出成功"},
		{"英文", "en-US", "en-us", "Exit successful"},
		{"下划线形态", "en_US", "en-us", "Exit successful"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := i18nGet(newI18nEngine(config.DefaultI18n()), LocaleHeader, tc.lang)
			body := w.Body.String()

			for _, field := range []string{
				`"fromGin":"` + tc.wantLoc + `"`,
				`"fromCtx":"` + tc.wantLoc + `"`,
				`"helper":"` + tc.wantLoc + `"`,
				`"msg":"` + tc.wantMsg + `"`,
			} {
				if !strings.Contains(body, field) {
					t.Errorf("body = %s\n应包含 %s", body, field)
				}
			}
		})
	}
}

// TestI18nFallsBackToDefault 不带语言头时回落默认语言。
func TestI18nFallsBackToDefault(t *testing.T) {
	w := i18nGet(newI18nEngine(config.DefaultI18n()), LocaleHeader, "")
	if !strings.Contains(w.Body.String(), `"msg":"退出成功"`) {
		t.Errorf("body = %s, 无语言头时应回落中文", w.Body.String())
	}
}

// TestI18nReadsContentLanguageNotAcceptLanguage 读的是 content-language，不是 Accept-Language。
func TestI18nReadsContentLanguageNotAcceptLanguage(t *testing.T) {
	r := newI18nEngine(config.DefaultI18n())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Language", "en-US")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), `"msg":"退出成功"`) {
		t.Errorf("body = %s\nAccept-Language 不应被采纳", w.Body.String())
	}
}

// TestI18nHeaderNameCaseInsensitive 头名大小写不敏感。
func TestI18nHeaderNameCaseInsensitive(t *testing.T) {
	for _, name := range []string{"content-language", "Content-Language", "CONTENT-LANGUAGE"} {
		w := i18nGet(newI18nEngine(config.DefaultI18n()), name, "en-US")
		if !strings.Contains(w.Body.String(), `"msg":"Exit successful"`) {
			t.Errorf("头名 %q 未被识别: %s", name, w.Body.String())
		}
	}
}

// TestI18nMalformedHeaderDoesNotFailRequest 非法语言标记不得拒绝请求，只回落默认语言。
func TestI18nMalformedHeaderDoesNotFailRequest(t *testing.T) {
	for _, bad := range []string{
		"zh-CN;q=0.9",
		"中文",
		"zh CN",
		strings.Repeat("a", 200),
	} {
		w := i18nGet(newI18nEngine(config.DefaultI18n()), LocaleHeader, bad)
		if w.Code != http.StatusOK {
			t.Errorf("语言头 %q 导致状态码 %d, 应仍为 200", bad, w.Code)
		}
		if !strings.Contains(w.Body.String(), `"msg":"退出成功"`) {
			t.Errorf("语言头 %q 应回落中文, 实际 %s", bad, w.Body.String())
		}
	}
}

// TestI18nRejectsHeaderInjection 带 CR/LF 的语言头不得进响应头。
func TestI18nRejectsHeaderInjection(t *testing.T) {
	w := i18nGet(newI18nEngine(config.DefaultI18n()), LocaleHeader, "zh-CN\r\nX-Injected: 1")

	if got := w.Header().Get("X-Injected"); got != "" {
		t.Errorf("注入的头被写出: X-Injected = %q", got)
	}
	if got := w.Header().Get("Content-Language"); strings.ContainsAny(got, "\r\n") {
		t.Errorf("Content-Language 含控制字符: %q", got)
	}
}

// TestI18nEchoesContentLanguageHeader 响应头回显本次实际生效的语言。
func TestI18nEchoesContentLanguageHeader(t *testing.T) {
	cases := map[string]string{
		"zh-CN":      "zh-cn",
		"en-US":      "en-us",
		"":           string(i18n.DefaultLocale),
		"zh-Hans-CN": "zh-hans-cn",
	}
	for in, want := range cases {
		w := i18nGet(newI18nEngine(config.DefaultI18n()), LocaleHeader, in)
		if got := w.Header().Get("Content-Language"); got != want {
			t.Errorf("语言头 %q → 响应 Content-Language = %q, 期望 %q", in, got, want)
		}
	}
}

// TestI18nHeaderSetBeforeHandlerWritesBody 响应头必须在 c.Next() 之前写。
func TestI18nHeaderSetBeforeHandlerWritesBody(t *testing.T) {
	r := gin.New()
	r.Use(I18n())
	r.GET("/stream", func(c *gin.Context) {
		c.String(http.StatusOK, "already written")
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set(LocaleHeader, "en-US")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Language"); got != "en-us" {
		t.Errorf("handler 写过 body 后 Content-Language = %q, 期望 en-us（应在 Next 之前写）", got)
	}
}

// TestI18nWithCustomConfig 自定义配置：换头名、换默认语言。
func TestI18nWithCustomConfig(t *testing.T) {
	cfg := config.I18nConfig{Header: "Accept-Language", Default: i18n.LocaleEnUS}
	r := newI18nEngine(cfg)

	w := i18nGet(r, "Accept-Language", "zh-CN")
	if !strings.Contains(w.Body.String(), `"msg":"退出成功"`) {
		t.Errorf("自定义头名未生效: %s", w.Body.String())
	}

	w = i18nGet(r, "Accept-Language", "")
	if !strings.Contains(w.Body.String(), `"msg":"Exit successful"`) {
		t.Errorf("自定义默认语言未生效: %s", w.Body.String())
	}
}

// TestI18nZeroConfigUsesDefaults 零值配置必须能用，回落到包级默认。
func TestI18nZeroConfigUsesDefaults(t *testing.T) {
	w := i18nGet(newI18nEngine(config.I18nConfig{}), LocaleHeader, "en-US")
	if !strings.Contains(w.Body.String(), `"msg":"Exit successful"`) {
		t.Errorf("零值配置应回落默认头名与默认语言: %s", w.Body.String())
	}
}

// TestLocaleFromFallback LocaleFrom 对 nil 与未挂中间件的 context 都返回默认语言。
func TestLocaleFromFallback(t *testing.T) {
	if got := LocaleFrom(nil); got != i18n.DefaultLocale {
		t.Errorf("LocaleFrom(nil) = %q, 期望 %q", got, i18n.DefaultLocale)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if got := LocaleFrom(c); got != i18n.DefaultLocale {
		t.Errorf("未挂中间件时 LocaleFrom = %q, 期望 %q", got, i18n.DefaultLocale)
	}
}

// TestI18nCoexistsWithTraceID I18n 与链路上其他中间件共存时互不干扰。
func TestI18nCoexistsWithTraceID(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.Use(I18n())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"trace":  TraceIDFrom(c.Request.Context()),
			"locale": string(i18n.FromContext(c.Request.Context())),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(LocaleHeader, "en-US")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"locale":"en-us"`) {
		t.Errorf("body = %s, 语言被 TraceID 覆盖了", body)
	}
	if strings.Contains(body, `"trace":""`) {
		t.Errorf("body = %s, 链路 id 被 I18n 覆盖了", body)
	}
}
