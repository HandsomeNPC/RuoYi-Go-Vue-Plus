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
//
// 复用 trace_test.go 的 captureLog（它顺带把 log.Flags 清零，
// 日志里不会混进时间戳，断言只面对中间件自己写的内容）。
func runCapturingLog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	fn()
	return buf.String()
}

// newAccessLogEngine 构造 TraceID + RepeatableBody + AccessLog 的引擎。
//
// 三个一起挂是刻意的：AccessLog 的 JSON 入参依赖 RepeatableBody 的缓存，
// 日志前缀依赖 TraceID，单挂 AccessLog 测不出真实注册顺序下的行为。
func newAccessLogEngine(cfg config.AccessLog) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(TraceID())
	r.Use(RepeatableBody())
	r.Use(AccessLogWithConfig(cfg))

	h := func(c *gin.Context) {
		// 读一次 body，确认 AccessLog 打完日志后 handler 仍能绑参。
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

// 一次请求必须打「开始」「结束」两行，缺一不可 ——
// 只有结束行的话，卡死的请求在日志里什么都不会留下。
func TestAccessLogStartAndEnd(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
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

// 无参数时走「无参数」分支，而不是打一个空的 [] 或 {}。
func TestAccessLogNoParam(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodGet, "/test", "", "")

	if !strings.Contains(out, "无参数") {
		t.Errorf("应打「无参数」, got=%q", out)
	}
}

// 核心用例：JSON body 里的密码字段必须被摘掉，其余字段保留。
func TestAccessLogSanitizesJSONBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
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

// 嵌套结构和数组里的敏感字段同样要摘掉 ——
// 只删顶层的话，{"user":{"password":"x"}} 就漏了。
func TestAccessLogSanitizesNested(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodPost, "/test", "application/json",
		`{"list":[{"name":"a","password":"leak1"}],"user":{"newPassword":"leak2"}}`)

	if strings.Contains(out, "leak1") || strings.Contains(out, "leak2") {
		t.Errorf("嵌套/数组中的敏感字段泄漏, got=%q", out)
	}
	if !strings.Contains(out, "\"name\"") {
		t.Errorf("非敏感字段被误删, got=%q", out)
	}
}

// 雪花 id 是 19 位整数，不能被 float64 抹掉尾数 ——
// 打进日志的假 id 拿去查库查不到，比不打还误导人。
func TestAccessLogKeepsLargeIntegerPrecision(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodPost, "/test", "application/json",
		`{"userId":1761100000000000001}`)

	if !strings.Contains(out, "1761100000000000001") {
		t.Errorf("雪花 id 精度丢失, got=%q", out)
	}
}

// 非法 JSON 且含敏感字段名时必须整段丢弃 ——
// 这是相对 Java 侧刻意收紧的一处（那边直接回原文，密码明文进日志）。
func TestAccessLogDropsUnparsableSensitiveBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodPost, "/test", "application/json",
		`{"username":"admin","password":"admin123`) // 少个右括号

	if strings.Contains(out, "admin123") {
		t.Errorf("非法 JSON 中的密码泄漏, got=%q", out)
	}
	if !strings.Contains(out, msgSensitiveParamOmitted) {
		t.Errorf("应整段省略, got=%q", out)
	}
}

// 非法 JSON 但不含敏感字段时保留原文，对齐 Java 的兜底行为。
func TestAccessLogKeepsUnparsableSafeBody(t *testing.T) {
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodPost, "/test", "application/json", `{"name":"admin`)

	if !strings.Contains(out, "admin") {
		t.Errorf("无敏感字段的非法 JSON 应保留原文, got=%q", out)
	}
}

// 查询串里的敏感参数同样要摘掉，且不能因为打日志就把参数从请求里删掉。
func TestAccessLogSanitizesQueryParams(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	var gotPassword string
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))
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

// 打日志时不能把 URL 的原始查询串直接输出，否则脱敏形同虚设。
func TestAccessLogDoesNotLeakRawQueryString(t *testing.T) {
	r := gin.New()
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	out := runCapturingLog(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/test?password=admin123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	})

	if strings.Contains(out, "admin123") {
		t.Errorf("原始查询串被打进日志, got=%q", out)
	}
}

// 超长参数必须截断，且带上截断标记。
func TestAccessLogTruncatesLongParam(t *testing.T) {
	long := strings.Repeat("a", 5000)
	out := accessLogDo(t, newAccessLogEngine(config.DefaultMiddleware().AccessLog),
		http.MethodPost, "/test", "application/json", `{"name":"`+long+`"}`)

	if !strings.Contains(out, truncatedSuffix) {
		t.Errorf("超长参数未截断, got 长度=%d", len(out))
	}
	if strings.Contains(out, strings.Repeat("a", 4100)) {
		t.Errorf("截断长度超出上限")
	}
}

// 按字符截断而非字节：中文从中间劈开会在日志里变成乱码。
func TestLimitParamCutsByRune(t *testing.T) {
	got := limitParam(strings.Repeat("中", 10), 3)

	want := "中中中" + truncatedSuffix
	if got != want {
		t.Errorf("limitParam = %q, want %q", got, want)
	}
}

// 恰好等于上限时不截断，不留多余标记。
func TestLimitParamExactLength(t *testing.T) {
	s := strings.Repeat("中", 4)
	if got := limitParam(s, 4); got != s {
		t.Errorf("limitParam = %q, want 原样返回", got)
	}
}

// SkipPaths 里的路径完全不打日志 —— 探针每几秒一次，会把有用日志冲走。
func TestAccessLogSkipPaths(t *testing.T) {
	cfg := config.DefaultMiddleware().AccessLog
	cfg.SkipPaths = []string{"/health"}
	r := newAccessLogEngine(cfg)

	if out := accessLogDo(t, r, http.MethodGet, "/health", "", ""); out != "" {
		t.Errorf("跳过的路径不应打日志, got=%q", out)
	}
	if out := accessLogDo(t, r, http.MethodGet, "/test", "", ""); out == "" {
		t.Error("未跳过的路径应正常打日志")
	}
}

// handler panic 时结束行仍要打出来 —— 靠 defer 保证，
// 对齐 afterCompletion 在 ex != null 时同样被调用。
func TestAccessLogEndLineOnPanic(t *testing.T) {
	r := gin.New()
	r.Use(Recover())
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))
	r.GET("/test", func(c *gin.Context) { panic("boom") })

	out := runCapturingLog(t, func() {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	})

	if !strings.Contains(out, "结束请求") {
		t.Errorf("panic 后缺少结束行, got=%q", out)
	}
}

// AccessLog 读过 body 之后，handler 必须还能绑到参数。
func TestAccessLogDoesNotConsumeBody(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.Use(RepeatableBody())
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))

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

// 没挂 RepeatableBody 时只能打空参数，绝不能回头去读 c.Request.Body。
func TestAccessLogWithoutRepeatableBody(t *testing.T) {
	r := gin.New()
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))

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

// 日志行必须带 traceId 前缀，否则并发下开始行和结束行没法配对。
func TestAccessLogHasTraceID(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.Use(AccessLogWithConfig(config.DefaultMiddleware().AccessLog))
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
