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

// newCORSEngine 构造只挂 CORS 的引擎。
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

// TestDefaultCORSConfigMatchesJava 默认配置应与既定默认值一致。
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

// TestCORSEchoesOriginNotWildcard 配 "*" 也必须回显具体 Origin。
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

// TestCORSPreflightDoesNotReachHandler 预检请求必须在中间件内终止。
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

// TestCORSPreflightEchoesRequestedMethodAndHeaders 配 "*" 时回显请求的方法与头。
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

// TestCORSActualRequestOmitsPreflightHeaders 实际请求不带预检专用的三个头。
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

// TestCORSSameOriginNoHeaders 同源请求不加任何 CORS 头但仍放行。
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

// TestCORSAlwaysSetsVary Vary 三个头无条件加（含同源请求）。
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

// TestCORSPlainOptionsIsNotPreflight 普通 OPTIONS 不是预检，应走后续链路。
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

// TestCORSRejectsDisallowedOrigin 白名单外的来源返 403，不带 Allow-Origin 头。
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

// TestCORSMethodWhitelist 显式方法白名单：命中放行，未命中 403。
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

// TestCORSHeaderWhitelistRejectsAny 显式头白名单：任一头不合规即整体拒绝。
func TestCORSHeaderWhitelistRejectsAny(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.AllowedHeaders = []string{"Content-Type", "Authorization"}
	r, _ := newCORSEngine(cfg)

	// 头名大小写不敏感。
	if got, want := preflight(r, "https://a.com", http.MethodPost, "content-type, AUTHORIZATION").Code,
		http.StatusOK; got != want {
		t.Errorf("合规头预检 HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := preflight(r, "https://a.com", http.MethodPost, "content-type, x-evil").Code,
		http.StatusForbidden; got != want {
		t.Errorf("含非法头的预检 HTTP 状态码 = %d, want %d", got, want)
	}
}

// TestCORSExposedHeaders ExposedHeaders 配了才输出。
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

// TestCORSCredentialsDisabled AllowCredentials=false 时不输出该头。
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

// TestCORSZeroMaxAge MaxAgeSeconds 为 0 时不输出该头。
func TestCORSZeroMaxAge(t *testing.T) {
	cfg := config.DefaultMiddleware().CORS
	cfg.MaxAgeSeconds = 0
	r, _ := newCORSEngine(cfg)

	w := preflight(r, "https://a.com", http.MethodPost, "")

	if got := w.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("Access-Control-Max-Age = %q, MaxAgeSeconds=0 时应不输出", got)
	}
}

// TestWildcardMatch 通配匹配的边界：前缀/后缀必须贴边。
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
		{"https://Admin.Example.com", "https://admin.example.com", true},

		{"https://*.example.com", "https://admin.example.com", true},
		{"https://*.example.com", "https://a.b.example.com", true},
		{"https://*.example.com", "https://example.com", false},
		{"https://*.example.com", "http://admin.example.com", false},

		// 后缀必须贴着结尾。
		{"https://*.example.com", "https://admin.example.com.evil.com", false},
		// 前缀必须贴着开头。
		{"https://*.example.com", "http://x.https://a.example.com", false},

		{"http://localhost:*", "http://localhost:8080", true},
		{"http://localhost:*", "http://localhost:", true},
		{"http://localhost:*", "http://127.0.0.1:8080", false},

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

// TestSplitHeaderList 拆分逗号分隔的头列表，去空白并丢掉空项。
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

// TestCORSHeadersSurvivePanic 业务 panic 时跨域头不能丢。
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
