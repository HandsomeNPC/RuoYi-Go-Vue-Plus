package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// newCORSEngine 构造只挂 CORS 的引擎，业务 handler 记录自己是否被执行到。
//
// reached 用来验证「预检不透给业务路由」这条约束 —— 光看响应码看不出来。
func newCORSEngine(cfg config.CORS) (*gin.Engine, *bool) {
	reached := false
	r := gin.New()
	r.Use(CORSWithConfig(cfg))
	h := func(c *gin.Context) {
		reached = true
		c.String(http.StatusOK, "ok")
	}
	r.GET("/test", h)
	r.POST("/test", h)
	return r, &reached
}

// preflight 发一次预检请求。
func preflight(r *gin.Engine, origin, method, headers string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	if headers != "" {
		req.Header.Set("Access-Control-Request-Headers", headers)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// actual 发一次带 Origin 的实际（非预检）请求。
func actual(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/test", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 默认配置必须与 Java 侧 CorsProperties 的字段初始值一致 ——
// 原项目 yaml 里没有 web.cors，生效的就是这些默认值。
func TestDefaultCORSConfigMatchesJava(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS

	if !cfg.AllowCredentials {
		t.Error("AllowCredentials 应为 true")
	}
	if got, want := cfg.MaxAge(), 1800*time.Second; got != want {
		t.Errorf("MaxAge() = %v, want %v", got, want)
	}
	for name, list := range map[string][]string{
		"AllowedOriginPatterns": cfg.AllowedOriginPatterns,
		"AllowedMethods":        cfg.AllowedMethods,
		"AllowedHeaders":        cfg.AllowedHeaders,
	} {
		if len(list) != 1 || list[0] != "*" {
			t.Errorf("%s = %v, want [*]", name, list)
		}
	}
}

// 这是本文件最重要的一条：配 "*" 也必须回显具体 Origin。
// allowCredentials=true 配 Origin: * 是浏览器非法组合，带凭证的请求会全挂。
func TestCORSEchoesOriginNotWildcard(t *testing.T) {
	r, _ := newCORSEngine(config.DefaultMiddleware().CORS)

	w := actual(r, http.MethodGet, "https://admin.example.com")

	if got, want := w.Header().Get("Access-Control-Allow-Origin"), "https://admin.example.com"; got != want {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q（不能回显 *）", got, want)
	}
	if got, want := w.Header().Get("Access-Control-Allow-Credentials"), "true"; got != want {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, want)
	}
}

// 预检请求必须在中间件里就结束，不能进业务 handler。
func TestCORSPreflightDoesNotReachHandler(t *testing.T) {
	r, reached := newCORSEngine(config.DefaultMiddleware().CORS)

	w := preflight(r, "https://admin.example.com", http.MethodPost, "content-type, authorization")

	if *reached {
		t.Error("预检请求进入了业务 handler，应在中间件内终止")
	}
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := w.Header().Get("Access-Control-Max-Age"), "1800"; got != want {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, want)
	}
}

// 配 "*" 时 Allow-Methods 回显请求的方法，对齐 checkHttpMethod
// 在 ALL 时返回 singletonList(requestMethod) 的行为。
func TestCORSPreflightEchoesRequestedMethodAndHeaders(t *testing.T) {
	r, _ := newCORSEngine(config.DefaultMiddleware().CORS)

	w := preflight(r, "https://admin.example.com", http.MethodDelete, "content-type, authorization")

	if got, want := w.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete; got != want {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, want)
	}
	if got, want := w.Header().Get("Access-Control-Allow-Headers"), "content-type, authorization"; got != want {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, want)
	}
}

// 实际请求不该带预检专用的三个头。
func TestCORSActualRequestOmitsPreflightHeaders(t *testing.T) {
	r, reached := newCORSEngine(config.DefaultMiddleware().CORS)

	w := actual(r, http.MethodGet, "https://admin.example.com")

	if !*reached {
		t.Error("实际请求未到达业务 handler")
	}
	for _, h := range []string{
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Max-Age",
	} {
		if got := w.Header().Get(h); got != "" {
			t.Errorf("实际请求不应带 %s, 实际 = %q", h, got)
		}
	}
}

// 同源请求（无 Origin 头）不加任何 CORS 头，但仍要放行。
func TestCORSSameOriginNoHeaders(t *testing.T) {
	r, reached := newCORSEngine(config.DefaultMiddleware().CORS)

	w := actual(r, http.MethodGet, "")

	if !*reached {
		t.Error("同源请求未到达业务 handler")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("同源请求不应带 Access-Control-Allow-Origin, 实际 = %q", got)
	}
}

// Vary 三个头无条件加（含同源请求），否则 CDN 会把 A 站的跨域头缓存给 B 站。
func TestCORSAlwaysSetsVary(t *testing.T) {
	r, _ := newCORSEngine(config.DefaultMiddleware().CORS)

	for _, origin := range []string{"", "https://admin.example.com"} {
		w := actual(r, http.MethodGet, origin)
		vary := strings.Join(w.Header().Values("Vary"), ", ")
		for _, want := range []string{
			"Origin",
			"Access-Control-Request-Method",
			"Access-Control-Request-Headers",
		} {
			if !strings.Contains(vary, want) {
				t.Errorf("origin=%q 时 Vary = %q, 应包含 %q", origin, vary, want)
			}
		}
	}
}

// 普通 OPTIONS（不带 Access-Control-Request-Method）不是预检，
// 应正常走后续链路 —— 对齐 CorsUtils.isPreFlightRequest 的双条件判断。
func TestCORSPlainOptionsIsNotPreflight(t *testing.T) {
	r := gin.New()
	r.Use(CORSWithConfig(config.DefaultMiddleware().CORS))
	reached := false
	r.OPTIONS("/test", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !reached {
		t.Error("不带 ACRM 的 OPTIONS 被误判成预检")
	}
	if got, want := w.Code, http.StatusNoContent; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
}

// 白名单外的来源要 403，且不能带 Allow-Origin 头。
func TestCORSRejectsDisallowedOrigin(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.AllowedOriginPatterns = []string{"https://admin.example.com"}
	r, reached := newCORSEngine(cfg)

	w := actual(r, http.MethodGet, "https://evil.com")

	if *reached {
		t.Error("非法来源的请求进入了业务 handler")
	}
	if got, want := w.Code, http.StatusForbidden; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("拒绝时不应带 Access-Control-Allow-Origin, 实际 = %q", got)
	}
}

// 显式方法白名单：命中放行，未命中 403。
func TestCORSMethodWhitelist(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.AllowedMethods = []string{"GET", "POST"}
	r, _ := newCORSEngine(cfg)

	if got, want := preflight(r, "https://a.com", http.MethodPost, "").Code, http.StatusOK; got != want {
		t.Errorf("POST 预检 HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := preflight(r, "https://a.com", http.MethodDelete, "").Code, http.StatusForbidden; got != want {
		t.Errorf("DELETE 预检 HTTP 状态码 = %d, want %d", got, want)
	}
}

// 显式头白名单：任一头不合规即整体拒绝，不是过滤掉那几个。
func TestCORSHeaderWhitelistRejectsAny(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.AllowedHeaders = []string{"Content-Type", "Authorization"}
	r, _ := newCORSEngine(cfg)

	// 头名大小写不敏感，浏览器发的是小写。
	if got, want := preflight(r, "https://a.com", http.MethodPost, "content-type, AUTHORIZATION").Code,
		http.StatusOK; got != want {
		t.Errorf("合规头预检 HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := preflight(r, "https://a.com", http.MethodPost, "content-type, x-evil").Code,
		http.StatusForbidden; got != want {
		t.Errorf("含非法头的预检 HTTP 状态码 = %d, want %d", got, want)
	}
}

// ExposedHeaders 配了才输出 —— TraceID 落地后要靠它把 X-Request-Id 透给前端。
func TestCORSExposedHeaders(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.ExposedHeaders = []string{"X-Request-Id", "Content-Disposition"}
	r, _ := newCORSEngine(cfg)

	w := actual(r, http.MethodGet, "https://a.com")

	if got, want := w.Header().Get("Access-Control-Expose-Headers"),
		"X-Request-Id, Content-Disposition"; got != want {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, want)
	}
}

// AllowCredentials=false 时不能输出该头（输出 "false" 也不行，
// 浏览器只认这个头存在与否）。
func TestCORSCredentialsDisabled(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.AllowCredentials = false
	r, _ := newCORSEngine(cfg)

	w := actual(r, http.MethodGet, "https://a.com")

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, 应完全不输出", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("Access-Control-Allow-Origin 仍应输出")
	}
}

// MaxAgeSeconds 为 0 时不输出该头，让浏览器用自己的默认值。
func TestCORSZeroMaxAge(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.MaxAgeSeconds = 0
	r, _ := newCORSEngine(cfg)

	w := preflight(r, "https://a.com", http.MethodPost, "")

	if got := w.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("Access-Control-Max-Age = %q, MaxAgeSeconds=0 时应不输出", got)
	}
}

// 通配匹配的边界：前缀/后缀必须贴边，否则 *.example.com
// 会被 evil.com 拼出来的域名蒙过去。
func TestWildcardMatch(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "https://anything.com", true},
		{"*", "", true},
		{"https://admin.example.com", "https://admin.example.com", true},
		{"https://admin.example.com", "https://other.example.com", false},
		// 大小写不敏感：Origin 的 scheme/host 按 RFC 6454 不区分大小写。
		{"https://Admin.Example.com", "https://admin.example.com", true},

		{"https://*.example.com", "https://admin.example.com", true},
		{"https://*.example.com", "https://a.b.example.com", true},
		{"https://*.example.com", "https://example.com", false},
		// 协议不同不能放行 —— 混用 http 会让凭证明文传输。
		{"https://*.example.com", "http://admin.example.com", false},

		// 关键的绕过防护：后缀必须贴着结尾。
		{"https://*.example.com", "https://admin.example.com.evil.com", false},
		// 前缀必须贴着开头。
		{"https://*.example.com", "http://x.https://a.example.com", false},

		{"http://localhost:*", "http://localhost:8080", true},
		{"http://localhost:*", "http://localhost:", true},
		{"http://localhost:*", "http://127.0.0.1:8080", false},

		// 多个通配符按顺序匹配。
		{"https://*.*.example.com", "https://a.b.example.com", true},
		{"https://*-test.example.com", "https://admin-test.example.com", true},
		{"https://*-test.example.com", "https://admin-prod.example.com", false},
	}

	for _, tt := range tests {
		if got := wildcardMatch(tt.pattern, tt.s); got != tt.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
		}
	}
}

// 浏览器发的 ACRH 是 "a, b" 形式，拆分要去空白并丢掉空项。
func TestSplitHeaderList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"content-type", []string{"content-type"}},
		{"content-type, authorization", []string{"content-type", "authorization"}},
		{" content-type ,authorization ", []string{"content-type", "authorization"}},
		{"content-type,,authorization", []string{"content-type", "authorization"}},
	}

	for _, tt := range tests {
		got := splitHeaderList(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitHeaderList(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitHeaderList(%q) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

// CORS 必须能在 Recover 之后正常工作，且业务 panic 时
// 跨域头不能丢 —— 否则前端拿到的是不带 CORS 头的 500，
// 只能看到「网络错误」而非真实错误信息。
func TestCORSHeadersSurvivePanic(t *testing.T) {
	r := gin.New()
	r.Use(Recover())
	r.Use(CORSWithConfig(config.DefaultMiddleware().CORS))
	r.GET("/test", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Header().Get("Access-Control-Allow-Origin"), "https://admin.example.com"; got != want {
		t.Errorf("panic 后 Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
}
