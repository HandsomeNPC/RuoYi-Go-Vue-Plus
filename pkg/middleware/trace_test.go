package middleware

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTraceEngine 构造只挂 TraceID 的引擎，业务 handler 把它在两处
// 上下文里看到的 id 回写出来，供断言比对。
//
// 分别取 gin.Context 和 request 的 context.Context 是刻意的：
// 两条取值路径都得能拿到同一个 id，前者给 handler，后者给 service 层。
func newTraceEngine(cfg TraceIDConfig) *gin.Engine {
	r := gin.New()
	r.Use(TraceIDWithConfig(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"fromGin": c.GetString(TraceIDKey),
			"fromCtx": TraceIDFrom(c.Request.Context()),
		})
	})
	return r
}

// traceGet 发一次请求，可选带入站的链路 id 头。
func traceGet(r *gin.Engine, inbound string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if inbound != "" {
		req.Header.Set(TraceIDHeader, inbound)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 没有入站 id 时应生成一个，且响应头与两处上下文里的值三者一致 ——
// 不一致意味着日志里的 id 和前端看到的对不上，整个机制就失去意义。
func TestTraceIDGeneratesAndPropagates(t *testing.T) {
	w := traceGet(newTraceEngine(DefaultTraceIDConfig()), "")

	id := w.Header().Get(TraceIDHeader)
	if id == "" {
		t.Fatal("响应头未带链路 id")
	}
	body := w.Body.String()
	for _, field := range []string{`"fromGin":"` + id + `"`, `"fromCtx":"` + id + `"`} {
		if !strings.Contains(body, field) {
			t.Errorf("body = %s, 应包含 %s（上下文与响应头的 id 必须一致）", body, field)
		}
	}
}

// 每个请求必须拿到不同的 id，否则无法区分并发请求的日志。
func TestTraceIDUniquePerRequest(t *testing.T) {
	r := newTraceEngine(DefaultTraceIDConfig())

	seen := make(map[string]bool, 100)
	for range 100 {
		id := traceGet(r, "").Header().Get(TraceIDHeader)
		if seen[id] {
			t.Fatalf("链路 id 重复: %q", id)
		}
		seen[id] = true
	}
}

// 生成格式对齐 W3C trace-id / nginx $request_id：32 位小写十六进制。
func TestNewTraceIDFormat(t *testing.T) {
	id := NewTraceID()

	if len(id) != 32 {
		t.Errorf("NewTraceID() = %q, 长度应为 32", id)
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("NewTraceID() = %q, 应全为小写十六进制", id)
		}
	}
}

// 上游已生成 id 时必须沿用，否则同一次调用在各进程里 id 不同，链路断掉。
func TestTraceIDReusesInbound(t *testing.T) {
	inbound := "0af7651916cd43dd8448eb211c80319c"

	w := traceGet(newTraceEngine(DefaultTraceIDConfig()), inbound)

	if got := w.Header().Get(TraceIDHeader); got != inbound {
		t.Errorf("响应头 id = %q, want %q（应沿用入站 id）", got, inbound)
	}
	if !strings.Contains(w.Body.String(), `"fromCtx":"`+inbound+`"`) {
		t.Errorf("上下文未拿到入站 id, body = %s", w.Body.String())
	}
}

// TrustInbound=false 时必须忽略入站 id 自己生成 ——
// 进程直接暴露在公网时靠这个开关防止调用方给一万个请求发同一个 id。
func TestTraceIDDistrustInbound(t *testing.T) {
	cfg := DefaultTraceIDConfig()
	cfg.TrustInbound = false
	inbound := "attacker-supplied-id"

	w := traceGet(newTraceEngine(cfg), inbound)

	if got := w.Header().Get(TraceIDHeader); got == inbound {
		t.Error("TrustInbound=false 时不应沿用入站 id")
	} else if got == "" {
		t.Error("忽略入站 id 后应生成新 id")
	}
}

// 本文件最重要的一条：不合规的入站 id 必须丢弃并重新生成。
//
// 带 CR/LF 的 id 若原样写进响应头就是头注入，写进日志就是伪造日志行；
// 超长的能零成本撑爆日志。这些都来自外部，必须当不可信输入处理。
func TestTraceIDRejectsMaliciousInbound(t *testing.T) {
	cases := map[string]string{
		"CRLF 响应头注入": "abc\r\nX-Injected: evil",
		"换行日志注入":     "abc\ndef",
		"回车":         "abc\rdef",
		"超长":         strings.Repeat("a", traceIDMaxLength+1),
		"空格":         "abc def",
		"分号":         "abc;def",
		"非 ASCII":    "链路编号",
		"null 字节":    "abc\x00def",
		"CORS 头闭合字符": "abc\"def",
	}

	r := newTraceEngine(DefaultTraceIDConfig())
	for name, inbound := range cases {
		t.Run(name, func(t *testing.T) {
			w := traceGet(r, inbound)

			id := w.Header().Get(TraceIDHeader)
			if id == inbound {
				t.Fatalf("非法入站 id 被沿用: %q", inbound)
			}
			// 必须回落到一个干净的新 id，而不是空串或被过滤后的残余。
			if len(id) != 32 {
				t.Errorf("回落 id = %q, 应为 32 位新生成 id", id)
			}
			if w.Header().Get("X-Injected") != "" {
				t.Error("发生了响应头注入")
			}
		})
	}
}

// 合法格式要放过：十六进制、UUID(带横线)、base64url 风格都是常见的上游 id。
func TestSanitizeTraceIDAcceptsCommonFormats(t *testing.T) {
	valid := []string{
		"0af7651916cd43dd8448eb211c80319c",     // W3C / nginx
		"3f2504e0-4f89-11d3-9a0c-0305e82c3301", // UUID
		"a_b-C9",                               // base64url 风格
		strings.Repeat("a", traceIDMaxLength),  // 恰好到上限
	}

	for _, id := range valid {
		if got := sanitizeTraceID(id); got != id {
			t.Errorf("sanitizeTraceID(%q) = %q, 合法 id 应原样返回", id, got)
		}
	}
}

// TraceIDFrom 拿不到 id 时返回空串而非 panic —— 调用方多半在打日志，
// 不能因为少个 id 把请求搞挂。nil context 也要能扛住。
func TestTraceIDFromMissing(t *testing.T) {
	if got := TraceIDFrom(context.Background()); got != "" {
		t.Errorf("TraceIDFrom(空 context) = %q, want \"\"", got)
	}
	//nolint:staticcheck // 就是要测 nil 这个退化输入
	if got := TraceIDFrom(nil); got != "" {
		t.Errorf("TraceIDFrom(nil) = %q, want \"\"", got)
	}
}

// 响应头必须在 handler 写 body 之前落定：body 一开始输出，header 就发出去了，
// 事后 Set 会无声失效。这里让 handler 直接写 body 来验证顺序没写反。
func TestTraceIDHeaderSetBeforeHandlerWrites(t *testing.T) {
	r := gin.New()
	r.Use(TraceID())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := traceGet(r, "")

	if w.Header().Get(TraceIDHeader) == "" {
		t.Error("handler 写完 body 后响应头丢失，说明 Set 晚于 c.Next()")
	}
}

// 自定义头名要生效，且不能再读写默认头名。
func TestTraceIDCustomHeader(t *testing.T) {
	cfg := DefaultTraceIDConfig()
	cfg.Header = "X-Trace-Id"
	r := newTraceEngine(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-Id", "inbound-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Trace-Id"); got != "inbound-id" {
		t.Errorf("X-Trace-Id = %q, want %q", got, "inbound-id")
	}
	if got := w.Header().Get(TraceIDHeader); got != "" {
		t.Errorf("不应写默认头名, %s = %q", TraceIDHeader, got)
	}
}

// Header 留空要回落到默认头名，而不是写出一个空名字的头。
func TestTraceIDEmptyHeaderFallsBack(t *testing.T) {
	w := traceGet(newTraceEngine(TraceIDConfig{TrustInbound: true}), "")

	if w.Header().Get(TraceIDHeader) == "" {
		t.Errorf("Header 为空时应回落到 %s", TraceIDHeader)
	}
}

// TraceID 必须在 CORS 的 ExposedHeaders 里，否则跨域下浏览器挡住这个头，
// 前端拿不到 traceId 就无法和服务端日志对账。两处是配套的。
func TestTraceIDExposedByDefaultCORS(t *testing.T) {
	exposed := DefaultCORSConfig().ExposedHeaders

	for _, h := range exposed {
		if h == TraceIDHeader {
			return
		}
	}
	t.Errorf("DefaultCORSConfig().ExposedHeaders = %v, 应含 %s", exposed, TraceIDHeader)
}

// Recover 的日志要带上 traceId，否则排查时拿到 traceId 也搜不到异常那行 ——
// 这是 TraceID 相对原项目（只有 8 位错误编号）真正多出来的能力。
func TestRecoverLogsIncludeTraceID(t *testing.T) {
	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	r := gin.New()
	r.Use(Recover(), TraceID())
	r.GET("/test", func(c *gin.Context) { panic("boom") })

	w := traceGet(r, "")

	id := w.Header().Get(TraceIDHeader)
	if id == "" {
		t.Fatal("响应头未带链路 id")
	}
	if got := buf.String(); !strings.Contains(got, "["+id+"]") {
		t.Errorf("panic 日志未带 traceId %q, 日志 = %s", id, got)
	}
}

// 没挂 TraceID 时 Recover 仍要能正常打日志，不能留下空的 "[]" 占位。
func TestRecoverLogsWithoutTraceID(t *testing.T) {
	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	r := gin.New()
	r.Use(Recover())
	r.GET("/test", func(c *gin.Context) { panic("boom") })

	traceGet(r, "")

	if got := buf.String(); strings.Contains(got, "[]") {
		t.Errorf("无 traceId 时日志出现空占位, 日志 = %s", got)
	}
}

// captureLog 把标准库日志重定向到 buf，返回恢复函数。
func captureLog(buf *bytes.Buffer) func() {
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	}
}
