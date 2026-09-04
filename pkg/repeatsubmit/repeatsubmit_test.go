package repeatsubmit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/constant"
)

func init() { gin.SetMode(gin.TestMode) }

// newCtx 造一个指定方法、路径与请求体的 gin 上下文。
func newCtx(method, path, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	c.Request = r
	return c
}

// TestIntervalBelowMinimumPanics 间隔小于 1 秒应在注册期 panic。
func TestIntervalBelowMinimumPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("间隔 999ms 应 panic")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "不能小于 1 秒") {
			t.Errorf("panic 内容 = %v, 应提示间隔下限", r)
		}
	}()
	newOptions(999*time.Millisecond, defaultMessage)
}

// TestIntervalExactlyMinimumOK 恰好 1 秒应放行（边界是闭区间）。
func TestIntervalExactlyMinimumOK(t *testing.T) {
	o := newOptions(time.Second, defaultMessage)
	if got, want := o.interval, time.Second; got != want {
		t.Errorf("间隔 = %v, want %v", got, want)
	}
}

// TestGetPanicsBeforeInit 未初始化就取包级实例应 panic。
func TestGetPanicsBeforeInit(t *testing.T) {
	mu.Lock()
	prev := defaultSubmitter
	defaultSubmitter = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defaultSubmitter = prev
		mu.Unlock()
	})

	defer func() {
		if recover() == nil {
			t.Fatal("未 Init 就调 get() 应 panic")
		}
	}()
	get()
}

// TestCombineKeyPrefix 缓存键须以全局前缀 + 请求路径开头。
func TestCombineKeyPrefix(t *testing.T) {
	s := &Submitter{tokenName: "Authorization"}
	key := s.combineKey(newCtx("POST", "/system/user", `{"a":1}`))

	want := constant.RepeatSubmitKey + "/system/user"
	if !strings.HasPrefix(key, want) {
		t.Errorf("键 = %q, 应以 %q 开头", key, want)
	}
	// 前缀之后是 sha256 的十六进制指纹，定长 64。
	if got := len(strings.TrimPrefix(key, want)); got != 64 {
		t.Errorf("指纹长度 = %d, want 64", got)
	}
}

// TestCombineKeySameInputSameKey 同 token 同路径同入参必须算出同一个键，否则防不住重复提交。
func TestCombineKeySameInputSameKey(t *testing.T) {
	s := &Submitter{tokenName: "Authorization"}

	c1 := newCtx("POST", "/system/user", `{"a":1}`)
	c1.Request.Header.Set("Authorization", "tk")
	c2 := newCtx("POST", "/system/user", `{"a":1}`)
	c2.Request.Header.Set("Authorization", "tk")

	if s.combineKey(c1) != s.combineKey(c2) {
		t.Error("同 token 同路径同入参应算出相同的键")
	}
}

// TestCombineKeyVaries 路径、入参、query、token 任一不同，键都应不同。
func TestCombineKeyVaries(t *testing.T) {
	s := &Submitter{tokenName: "Authorization"}

	base := newCtx("POST", "/system/user", `{"a":1}`)
	base.Request.Header.Set("Authorization", "tk")
	baseKey := s.combineKey(base)

	withToken := func(c *gin.Context, tk string) *gin.Context {
		c.Request.Header.Set("Authorization", tk)
		return c
	}

	cases := []struct {
		name string
		ctx  *gin.Context
	}{
		{"路径不同", withToken(newCtx("POST", "/system/role", `{"a":1}`), "tk")},
		{"请求体不同", withToken(newCtx("POST", "/system/user", `{"a":2}`), "tk")},
		{"query 不同", withToken(newCtx("POST", "/system/user?x=1", `{"a":1}`), "tk")},
		{"token 不同", withToken(newCtx("POST", "/system/user", `{"a":1}`), "other")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if s.combineKey(tc.ctx) == baseKey {
				t.Errorf("%s 时键不应与基准相同", tc.name)
			}
		})
	}
}

// TestRequestBodyRestored 计算指纹后请求体必须还能被 handler 完整读出。
func TestRequestBodyRestored(t *testing.T) {
	c := newCtx("POST", "/system/user", `{"a":1}`)

	if got, want := string(requestBody(c)), `{"a":1}`; got != want {
		t.Errorf("指纹取到的 body = %q, want %q", got, want)
	}

	rest := make([]byte, 16)
	n, _ := c.Request.Body.Read(rest)
	if got, want := string(rest[:n]), `{"a":1}`; got != want {
		t.Errorf("handler 读到的 body = %q, want %q", got, want)
	}
}

// TestRequestParams 入参拼装：body 与 query 都有时用空格分隔。
func TestRequestParams(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		want string
	}{
		{"仅 body", "/u", `{"a":1}`, `{"a":1}`},
		{"仅 query", "/u?x=1&y=2", "", "x=1&y=2"},
		{"都有", "/u?x=1", `{"a":1}`, `{"a":1} x=1`},
		{"都无", "/u", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(requestParams(newCtx("POST", tc.path, tc.body)))
			if got != tc.want {
				t.Errorf("入参 = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseCode 响应体解析：只有合法的 R 结构才返回 ok。
func TestParseCode(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int
		wantOK   bool
	}{
		{"成功响应", `{"code":200,"msg":"操作成功","data":null}`, 200, true},
		{"失败响应", `{"code":500,"msg":"操作失败","data":null}`, 500, true},
		{"空响应体", "", 0, false},
		{"非 JSON", "hello", 0, false},
		{"JSON 但无 code", `{"msg":"x"}`, 0, false},
		{"文件流", "\x89PNG\r\n", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, ok := parseCode([]byte(tc.body))
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
		})
	}
}

// TestSucceeded 成功判定：c.Error / HTTP 非 2xx / 业务 code 非 200 都算失败。
func TestSucceeded(t *testing.T) {
	s := &Submitter{tokenName: "Authorization"}

	t.Run("业务成功", func(t *testing.T) {
		c := newCtx("POST", "/u", "")
		buf := &bodyWriter{ResponseWriter: c.Writer}
		buf.body.WriteString(`{"code":200,"msg":"操作成功","data":null}`)
		c.Writer = buf
		if !s.succeeded(c, buf) {
			t.Error("code=200 应判为成功")
		}
	})

	t.Run("业务失败", func(t *testing.T) {
		c := newCtx("POST", "/u", "")
		buf := &bodyWriter{ResponseWriter: c.Writer}
		buf.body.WriteString(`{"code":500,"msg":"操作失败","data":null}`)
		c.Writer = buf
		if s.succeeded(c, buf) {
			t.Error("code=500 应判为失败")
		}
	})

	t.Run("登记了错误", func(t *testing.T) {
		c := newCtx("POST", "/u", "")
		buf := &bodyWriter{ResponseWriter: c.Writer}
		c.Writer = buf
		_ = c.Error(http.ErrBodyNotAllowed)
		if s.succeeded(c, buf) {
			t.Error("c.Error 非空应判为失败")
		}
	})

	t.Run("非 R 结构按成功处理", func(t *testing.T) {
		c := newCtx("POST", "/u", "")
		buf := &bodyWriter{ResponseWriter: c.Writer}
		buf.body.WriteString("\x89PNG\r\n")
		c.Writer = buf
		if !s.succeeded(c, buf) {
			t.Error("非 R 结构(文件流)应判为成功,对照 Java instanceof 不匹配即保留键")
		}
	})
}

// TestBodyWriterFlush 缓冲的响应体 flush 后应原样写出。
func TestBodyWriterFlush(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	buf := &bodyWriter{ResponseWriter: c.Writer}
	buf.WriteString(`{"code":200}`)

	if w.Body.Len() != 0 {
		t.Error("flush 前不应写出任何内容")
	}
	buf.flush()
	if got, want := w.Body.String(), `{"code":200}`; got != want {
		t.Errorf("flush 后响应体 = %q, want %q", got, want)
	}
}
