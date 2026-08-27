package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// runCapturingLog 跑一次 fn 并返回期间打出的日志。
func runCapturingLog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	fn()
	return buf.String()
}

// newAccessLogEngine 构造 TraceID + RepeatableBody + AccessLog 的引擎。
func newAccessLogEngine(cfg config.AccessLogConfig) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(TraceID())
	r.Use(RepeatableBody())
	r.Use(AccessLogWithConfig(cfg))

	h := func(c *gin.Context) {
		var in map[string]any
		_ = c.ShouldBindJSON(&in)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
	r.POST("/test", h)
	r.GET("/test", h)
	r.GET("/health", h)
	return r
}

// accessLogDo 发一次请求并返回期间打出的日志。
func accessLogDo(t *testing.T, r *gin.Engine, method, target, contentType, body string) string {
	t.Helper()
	return runCapturingLog(t, func() {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		r.ServeHTTP(httptest.NewRecorder(), req)
	})
}

// TestAccessLogStartAndEnd 一次请求必须打「开始」「结束」两行。
func TestAccessLogStartAndEnd(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodGet, "/test", "", "")

	if !strings.Contains(out, "开始请求 => URL[GET /test]") {
		t.Errorf("缺少开始行, got=%q", out)
	}
	if !strings.Contains(out, "结束请求 => URL[GET /test]") {
		t.Errorf("缺少结束行, got=%q", out)
	}
	if !strings.Contains(out, "毫秒") {
		t.Errorf("结束行未打耗时, got=%q", out)
	}
}

// TestAccessLogNoParam 无参数时走「无参数」分支。
func TestAccessLogNoParam(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodGet, "/test", "", "")

	if !strings.Contains(out, "无参数") {
		t.Errorf("应打「无参数」, got=%q", out)
	}
}

// TestAccessLogSanitizesJSONBody JSON body 里的密码字段必须被摘掉，其余字段保留。
func TestAccessLogSanitizesJSONBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json;charset=UTF-8",
		`{"username":"admin","password":"admin123"}`)

	if strings.Contains(out, "admin123") {
		t.Errorf("密码泄漏进日志, got=%q", out)
	}
	if strings.Contains(out, "password") {
		t.Errorf("password 字段未被摘掉, got=%q", out)
	}
	if !strings.Contains(out, "admin") {
		t.Errorf("非敏感字段被误删, got=%q", out)
	}
	if !strings.Contains(out, "参数类型[json]") {
		t.Errorf("未识别为 json 请求, got=%q", out)
	}
}

// TestAccessLogSanitizesNested 嵌套结构和数组里的敏感字段同样要摘掉。
func TestAccessLogSanitizesNested(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json",
		`{"list":[{"name":"a","password":"leak1"}],"user":{"newPassword":"leak2"}}`)

	if strings.Contains(out, "leak1") || strings.Contains(out, "leak2") {
		t.Errorf("嵌套/数组中的敏感字段泄漏, got=%q", out)
	}
	if !strings.Contains(out, "\"name\"") {
		t.Errorf("非敏感字段被误删, got=%q", out)
	}
}

// TestAccessLogKeepsLargeIntegerPrecision 雪花 id 是 19 位整数，不能被 float64 抹掉尾数。
func TestAccessLogKeepsLargeIntegerPrecision(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json",
		`{"userId":1761100000000000001}`)

	if !strings.Contains(out, "1761100000000000001") {
		t.Errorf("雪花 id 精度丢失, got=%q", out)
	}
}

// TestAccessLogDropsUnparsableSensitiveBody 非法 JSON 且含敏感字段名时整段丢弃。
func TestAccessLogDropsUnparsableSensitiveBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json",
		`{"username":"admin","password":"admin123`)

	if strings.Contains(out, "admin123") {
		t.Errorf("非法 JSON 中的密码泄漏, got=%q", out)
	}
	if !strings.Contains(out, msgSensitiveParamOmitted) {
		t.Errorf("应整段省略, got=%q", out)
	}
}

// TestAccessLogKeepsUnparsableSafeBody 非法 JSON 但不含敏感字段时保留原文。
func TestAccessLogKeepsUnparsableSafeBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json", `{"name":"admin`)

	if !strings.Contains(out, "admin") {
		t.Errorf("无敏感字段的非法 JSON 应保留原文, got=%q", out)
	}
}

// TestAccessLogSanitizesQueryParams 查询串里的敏感参数同样要摘掉，且不能改动请求本身。
func TestAccessLogSanitizesQueryParams(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	var gotPassword string
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))
	r.GET("/test", func(c *gin.Context) {
		gotPassword = c.Query("password")
		c.Status(http.StatusOK)
	})

	out := runCapturingLog(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/test?username=admin&password=admin123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	})

	if strings.Contains(out, "admin123") {
		t.Errorf("查询串中的密码泄漏, got=%q", out)
	}
	if !strings.Contains(out, "参数类型[param]") {
		t.Errorf("未识别为 param 请求, got=%q", out)
	}
	// 日志中间件绝不能改动请求本身。
	if gotPassword != "admin123" {
		t.Errorf("handler 取到的 password = %q, 中间件不应改动请求", gotPassword)
	}
}

// TestAccessLogDoesNotLeakRawQueryString 打日志时不能把 URL 的原始查询串直接输出。
func TestAccessLogDoesNotLeakRawQueryString(t *testing.T) {
	r := gin.New()
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	out := runCapturingLog(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/test?password=admin123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	})

	if strings.Contains(out, "admin123") {
		t.Errorf("原始查询串被打进日志, got=%q", out)
	}
}

// TestAccessLogTruncatesLongParam 超长参数必须截断，且带上截断标记。
func TestAccessLogTruncatesLongParam(t *testing.T) {
	long := strings.Repeat("a", 5000)
	out := accessLogDo(t, newAccessLogEngine(config.DefaultAccessLog()),
		http.MethodPost, "/test", "application/json", `{"name":"`+long+`"}`)

	if !strings.Contains(out, truncatedSuffix) {
		t.Errorf("超长参数未截断, got 长度=%d", len(out))
	}
	if strings.Contains(out, strings.Repeat("a", 4100)) {
		t.Errorf("截断长度超出上限")
	}
}

// TestLimitParamCutsByRune 按字符截断而非字节。
func TestLimitParamCutsByRune(t *testing.T) {
	got := limitParam(strings.Repeat("中", 10), 3)

	want := "中中中" + truncatedSuffix
	if got != want {
		t.Errorf("limitParam = %q, want %q", got, want)
	}
}

// TestLimitParamExactLength 恰好等于上限时不截断。
func TestLimitParamExactLength(t *testing.T) {
	s := strings.Repeat("中", 4)
	if got := limitParam(s, 4); got != s {
		t.Errorf("limitParam = %q, want 原样返回", got)
	}
}

// TestAccessLogSkipPaths SkipPaths 里的路径完全不打日志。
func TestAccessLogSkipPaths(t *testing.T) {
	cfg := config.DefaultAccessLog()
	cfg.SkipPaths = []string{"/health"}
	r := newAccessLogEngine(cfg)

	if out := accessLogDo(t, r, http.MethodGet, "/health", "", ""); out != "" {
		t.Errorf("跳过的路径不应打日志, got=%q", out)
	}
	if out := accessLogDo(t, r, http.MethodGet, "/test", "", ""); out == "" {
		t.Error("未跳过的路径应正常打日志")
	}
}

// TestAccessLogEndLineOnPanic handler panic 时结束行仍要打出来。
func TestAccessLogEndLineOnPanic(t *testing.T) {
	r := gin.New()
	r.Use(Recover())
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))
	r.GET("/test", func(c *gin.Context) { panic("boom") })

	out := runCapturingLog(t, func() {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	})

	if !strings.Contains(out, "结束请求") {
		t.Errorf("panic 后缺少结束行, got=%q", out)
	}
}

// TestAccessLogDoesNotConsumeBody AccessLog 读过 body 之后 handler 必须还能绑到参数。
func TestAccessLogDoesNotConsumeBody(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.Use(RepeatableBody())
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))

	var bound string
	r.POST("/test", func(c *gin.Context) {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			t.Errorf("body 被日志中间件吃掉: %v", err)
		}
		bound = in.Name
		c.Status(http.StatusOK)
	})

	runCapturingLog(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"tom"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	})

	if bound != "tom" {
		t.Errorf("handler 绑到的 name = %q, want tom", bound)
	}
}

// TestAccessLogWithoutRepeatableBody 没挂 RepeatableBody 时只能打空参数，绝不回头读 c.Request.Body。
func TestAccessLogWithoutRepeatableBody(t *testing.T) {
	r := gin.New()
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))

	var bound string
	r.POST("/test", func(c *gin.Context) {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			t.Errorf("body 被日志中间件吃掉: %v", err)
		}
		bound = in.Name
		c.Status(http.StatusOK)
	})

	out := runCapturingLog(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"tom"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	})

	if bound != "tom" {
		t.Errorf("handler 绑到的 name = %q, want tom（body 不应被读走）", bound)
	}
	if !strings.Contains(out, "参数:[]") {
		t.Errorf("未缓存时应打空参数, got=%q", out)
	}
}

// TestAccessLogHasTraceID 日志行必须带 traceId 前缀，否则并发下两行没法配对。
func TestAccessLogHasTraceID(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.Use(AccessLogWithConfig(config.DefaultAccessLog()))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	var id string
	out := runCapturingLog(t, func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
		id = w.Header().Get(TraceIDHeader)
	})

	if id == "" {
		t.Fatal("响应头未带链路 id")
	}
	if strings.Count(out, "["+id+"]") != 2 {
		t.Errorf("开始行与结束行应都带 traceId %s, got=%q", id, out)
	}
}
