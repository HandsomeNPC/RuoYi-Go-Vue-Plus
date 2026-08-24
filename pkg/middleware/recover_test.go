package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// 中间件靠 log 打日志，测试里会刻意触发 panic/异常分支，
	// 不静音会把真实失败信息淹掉。
	log.SetOutput(io.Discard)

	// 无参构造函数（XSS() / I18n() 等）读 config.Get()，未 Load 过会 panic ——
	// 那是刻意的（启动期编排错误不该留到运行时），但测试得先把配置备好。
	//
	// 有意加载**真实**的 configs/*.yaml 而不是就地拼一份最小配置：
	// 这样一旦 yaml 里的中间件参数被改坏（exposedHeaders 漏了 X-Request-Id、
	// excludeUrls 拼错路径之类），本包断言默认行为的用例会直接失败，
	// 而不是等到线上才发现。application.yaml 缺 server 段过不了校验，
	// 所以带上 system.yaml。
	if err := config.Load(
		"../../configs/application.yaml", "../../configs/system.yaml"); err != nil {
		panic("middleware 测试无法加载配置: " + err.Error())
	}

	m.Run()
}

// newTestEngine 构造只挂 Recover 的引擎，handler 行为由 h 决定。
func newTestEngine(h gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.GET("/test", h)
	return r
}

// do 发一次请求并解出响应体。
func do(t *testing.T, r *gin.Engine) (*httptest.ResponseRecorder, response.R[any]) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

	var body response.R[any]
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("响应体不是合法 JSON: %v, body=%q", err, w.Body.String())
		}
	}
	return w, body
}

// panic 不能打挂进程，必须转成统一响应。
func TestRecoverPanic(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		panic("boom")
	})

	w, body := do(t, r)

	// HTTP 状态码恒为 200，业务码在响应体里 —— 对齐原项目，
	// 前端拦截器只认 body.code。
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := body.Code, response.CodeFail; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
	if got, want := body.Msg, msgUnknownError; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
}

// panic 值不是 string 时同样要兜住（最常见的是 runtime error）。
func TestRecoverPanicNilDeref(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		var p *int
		_ = *p // runtime error: invalid memory address
	})

	w, body := do(t, r)

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := body.Code, response.CodeFail; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
}

// 业务异常带业务码时，原样透出 code 与 msg。
func TestRecoverServiceErrorWithCode(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errs.NewCode(response.CodeUnauthorized, "客户端ID与Token不匹配"))
	})

	_, body := do(t, r)

	if got, want := body.Code, response.CodeUnauthorized; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
	if got, want := body.Msg, "客户端ID与Token不匹配"; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
}

// 业务异常未指定 code 时回落 500，
// 对齐 Java `code != null ? R.fail(code,msg) : R.fail(msg)`。
func TestRecoverServiceErrorWithoutCode(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errs.New("用户不存在"))
	})

	_, body := do(t, r)

	if got, want := body.Code, response.CodeFail; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
	if got, want := body.Msg, "用户不存在"; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
}

// 业务异常的消息要原样回前端，不能被兜底文案吞掉，也不该拼错误编号。
func TestRecoverServiceErrorMsgNotMasked(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errs.New("存在下级部门,不允许删除"))
	})

	_, body := do(t, r)

	if strings.Contains(body.Msg, "错误编号") {
		t.Errorf("业务异常不应附加错误编号, body.Msg = %q", body.Msg)
	}
	if got, want := body.Msg, "存在下级部门,不允许删除"; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
}

// 包装过的业务异常仍要被识别（errors.As 沿 Unwrap 链查找）。
func TestRecoverWrappedServiceError(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		wrapped := errors.Join(errors.New("上下文"), errs.NewCode(601, "警告"))
		_ = c.Error(wrapped)
	})

	_, body := do(t, r)

	if got, want := body.Code, response.CodeWarn; got != want {
		t.Errorf("包装后的业务异常未被识别, body.Code = %d, want %d", got, want)
	}
}

// 非业务错误必须脱敏：原始错误内容不能出现在响应里。
func TestRecoverGenericErrorIsMasked(t *testing.T) {
	const raw = "Error 1045: Access denied for user 'root'@'10.0.0.5'"
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errors.New(raw))
	})

	_, body := do(t, r)

	if strings.Contains(body.Msg, raw) || strings.Contains(body.Msg, "root") {
		t.Errorf("原始错误泄漏到响应, body.Msg = %q", body.Msg)
	}
	if !strings.HasPrefix(body.Msg, msgUnknownError) {
		t.Errorf("body.Msg = %q, 应以兜底文案开头", body.Msg)
	}
	// 带 8 位错误编号，便于和日志对账（对应 Java 的 randomNumbers(8)）。
	if !regexp.MustCompile(`\[错误编号: \d{8}]$`).MatchString(body.Msg) {
		t.Errorf("body.Msg = %q, 应以 8 位错误编号结尾", body.Msg)
	}
	if got, want := body.Code, response.CodeFail; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
}

// handler 正常返回时中间件不得改动响应。
func TestRecoverPassThrough(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok("ok"))
	})

	w, body := do(t, r)

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("HTTP 状态码 = %d, want %d", got, want)
	}
	if got, want := body.Code, response.CodeSuccess; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
	if got, want := body.Data, "ok"; got != want {
		t.Errorf("body.Data = %v, want %q", got, want)
	}
}

// 已写过响应时不得再覆盖 —— 否则流式输出（SSE/下载）的报文会被污染。
func TestRecoverDoesNotOverwriteWrittenResponse(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok("已发送"))
		// 响应已提交后才登记错误，此时无法再改 body。
		_ = c.Error(errors.New("事后才发现的错误"))
	})

	_, body := do(t, r)

	if got, want := body.Code, response.CodeSuccess; got != want {
		t.Errorf("已提交的响应被覆盖, body.Code = %d, want %d", got, want)
	}
	if got, want := body.Data, "已发送"; got != want {
		t.Errorf("body.Data = %v, want %q", got, want)
	}
}

// 多个错误时取最后一个，对齐 gin 的 Errors.Last 语义。
func TestRecoverUsesLastError(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errs.New("第一个"))
		_ = c.Error(errs.NewCode(403, "最后一个"))
	})

	_, body := do(t, r)

	if got, want := body.Msg, "最后一个"; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
	if got, want := body.Code, response.CodeForbidden; got != want {
		t.Errorf("body.Code = %d, want %d", got, want)
	}
}

// Detail 只进日志，绝不能回给前端。
func TestRecoverDetailNotLeaked(t *testing.T) {
	r := newTestEngine(func(c *gin.Context) {
		_ = c.Error(errs.New("保存失败").WithDetail("INSERT INTO sys_user ... duplicate key 'admin'"))
	})

	_, body := do(t, r)

	if strings.Contains(body.Msg, "INSERT INTO") || strings.Contains(body.Msg, "sys_user") {
		t.Errorf("Detail 泄漏到响应, body.Msg = %q", body.Msg)
	}
	if got, want := body.Msg, "保存失败"; got != want {
		t.Errorf("body.Msg = %q, want %q", got, want)
	}
}

// 错误编号必须是 8 位，且不能每次都一样（否则失去对账价值）。
func TestNewErrorID(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id := newErrorID()
		if len(id) != 8 {
			t.Fatalf("newErrorID() = %q, 长度应为 8", id)
		}
		for _, ch := range id {
			if ch < '0' || ch > '9' {
				t.Fatalf("newErrorID() = %q, 应全为数字", id)
			}
		}
		seen[id] = true
	}
	if len(seen) < 2 {
		t.Error("newErrorID() 恒定返回同一值")
	}
}
