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

	"ruoyi-go-vue-plus/pkg/config"
)

// newTraceEngine 构造只挂 TraceID 的引擎，回写两处上下文里看到的 id。
func newTraceEngine(cfg config.TraceID) *gin.Engine {
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

// TestTraceIDGeneratesAndPropagates 没有入站 id 时应生成一个，且三处一致。
func TestTraceIDGeneratesAndPropagates(t *testing.T) {
	w := traceGet(newTraceEngine(config.DefaultMiddleware().TraceID), "")

	id := w.Header().Get(TraceIDHeader)
	if id == "" {
		t.Fatal("响应头未带链路 id")
	}
	body := w.Body.String()
	for _, field := range []string{`"fromGin":"` + id + `"`, `"fromCtx":"` + id + `"`} {
		if !strings.Contains(body, field) {
			t.Errorf("body = %s, 应包含 %s", body, field)
		}
	}
}

// TestTraceIDUniquePerRequest 每个请求必须拿到不同的 id。
func TestTraceIDUniquePerRequest(t *testing.T) {
	r := newTraceEngine(config.DefaultMiddleware().TraceID)

	seen := make(map[string]bool, 100)
	for range 100 {
		id := traceGet(r, "").Header().Get(TraceIDHeader)
		if seen[id] {
			t.Fatalf("链路 id 重复: %q", id)
		}
		seen[id] = true
	}
}

// TestNewTraceIDFormat 生成格式为 32 位小写十六进制。
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

// TestTraceIDReusesInbound 上游已生成 id 时必须沿用。
func TestTraceIDReusesInbound(t *testing.T) {
	inbound := "0af7651916cd43dd8448eb211c80319c"

	w := traceGet(newTraceEngine(config.DefaultMiddleware().TraceID), inbound)

	if got := w.Header().Get(TraceIDHeader); got != inbound {
		t.Errorf("响应头 id = %q, want %q（应沿用入站 id）", got, inbound)
	}
	if !strings.Contains(w.Body.String(), `"fromCtx":"`+inbound+`"`) {
		t.Errorf("上下文未拿到入站 id, body = %s", w.Body.String())
	}
}

// TestTraceIDDistrustInbound TrustInbound=false 时必须忽略入站 id 自己生成。
func TestTraceIDDistrustInbound(t *testing.T) {
	cfg := config.DefaultMiddleware().TraceID
	cfg.TrustInbound = false
	inbound := "attacker-supplied-id"

	w := traceGet(newTraceEngine(cfg), inbound)

	if got := w.Header().Get(TraceIDHeader); got == inbound {
		t.Error("TrustInbound=false 时不应沿用入站 id")
	} else if got == "" {
		t.Error("忽略入站 id 后应生成新 id")
	}
}

// TestTraceIDRejectsMaliciousInbound 不合规的入站 id 必须丢弃并重新生成。
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

	r := newTraceEngine(config.DefaultMiddleware().TraceID)
	for name, inbound := range cases {
		t.Run(name, func(t *testing.T) {
			w := traceGet(r, inbound)

			id := w.Header().Get(TraceIDHeader)
			if id == inbound {
				t.Fatalf("非法入站 id 被沿用: %q", inbound)
			}
			// 必须回落到一个干净的新 id。
			if len(id) != 32 {
				t.Errorf("回落 id = %q, 应为 32 位新生成 id", id)
			}
			if w.Header().Get("X-Injected") != "" {
				t.Error("发生了响应头注入")
			}
		})
	}
}

// TestSanitizeTraceIDAcceptsCommonFormats 合法格式要放过：十六进制、UUID、base64url 风格。
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

// TestTraceIDFromMissing TraceIDFrom 拿不到 id 时返回空串而非 panic。
func TestTraceIDFromMissing(t *testing.T) {
	if got := TraceIDFrom(context.Background()); got != "" {
		t.Errorf("TraceIDFrom(空 context) = %q, want \"\"", got)
	}
	//nolint:staticcheck // 就是要测 nil 这个退化输入
	if got := TraceIDFrom(nil); got != "" {
		t.Errorf("TraceIDFrom(nil) = %q, want \"\"", got)
	}
}

// TestTraceIDHeaderSetBeforeHandlerWrites 响应头必须在 handler 写 body 之前落定。
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

// TestTraceIDCustomHeader 自定义头名要生效。
func TestTraceIDCustomHeader(t *testing.T) {
	cfg := config.DefaultMiddleware().TraceID
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

// TestTraceIDEmptyHeaderFallsBack Header 留空要回落到默认头名。
func TestTraceIDEmptyHeaderFallsBack(t *testing.T) {
	w := traceGet(newTraceEngine(config.TraceID{TrustInbound: true}), "")

	if w.Header().Get(TraceIDHeader) == "" {
		t.Errorf("Header 为空时应回落到 %s", TraceIDHeader)
	}
}

// TestTraceIDExposedByDefaultCORS TraceID 必须在 CORS 的 ExposedHeaders 里。
func TestTraceIDExposedByDefaultCORS(t *testing.T) {
	exposed := config.DefaultMiddleware().CORS.ExposedHeaders

	for _, h := range exposed {
		if h == TraceIDHeader {
			return
		}
	}
	t.Errorf("config.DefaultMiddleware().CORS.ExposedHeaders = %v, 应含 %s", exposed, TraceIDHeader)
}

// TestRecoverLogsIncludeTraceID Recover 的日志要带上 traceId。
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

// TestRecoverLogsWithoutTraceID 没挂 TraceID 时 Recover 仍要能正常打日志。
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
