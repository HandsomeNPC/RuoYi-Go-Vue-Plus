package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"

	"ruoyi-go-vue-plus/pkg/response"
)

// newBodyEngine 构造 RepeatableBody + 业务 handler 的引擎。
//
// handler 用 ShouldBindJSON 绑参，是为了验证「中间件读过 body 之后
// handler 仍能正常绑定」—— 这正是本中间件存在的全部意义。
// 同时回写中间件缓存里看到的 body，确认两条读取路径拿到的是同一份数据。
func newBodyEngine(cfg config.RepeatableBody) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBodyWithConfig(cfg))

	// 模拟 AccessLog：在业务 handler 之前读一次 body。
	var seenByMiddleware string
	r.Use(func(c *gin.Context) {
		seenByMiddleware = string(BodyBytes(c))
		c.Next()
	})

	r.POST("/test", func(c *gin.Context) {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusOK, gin.H{"bindErr": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"name":         in.Name,
			"seenByLogger": seenByMiddleware,
		})
	})
	return r
}

// bodyPost 发一次带 body 的请求。contentType 为空则不设该头。
func bodyPost(r *gin.Engine, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 核心用例：中间件读完 body 后，handler 必须还能绑到参数。
// 这条挂了说明整个中间件是负作用 —— 它把 body 吃掉了。
func TestRepeatableBodyAllowsRebind(t *testing.T) {
	body := `{"name":"张三"}`
	w := bodyPost(newBodyEngine(config.DefaultMiddleware().RepeatableBody), "application/json", body)

	var got struct {
		Name         string `json:"name"`
		SeenByLogger string `json:"seenByLogger"`
		BindErr      string `json:"bindErr"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v, body=%q", err, w.Body.String())
	}

	if got.BindErr != "" {
		t.Fatalf("handler 绑定失败: %s（body 被前面的中间件吃掉了）", got.BindErr)
	}
	if got.Name != "张三" {
		t.Errorf("handler 绑到 name=%q, 期望 %q", got.Name, "张三")
	}
	if got.SeenByLogger != body {
		t.Errorf("中间件读到 %q, 期望 %q（两条路径必须是同一份数据）", got.SeenByLogger, body)
	}
}

// ShouldBindBodyWith 要能复用缓存反复绑定不同结构体 ——
// 这是相对 Java 侧多做的一件事（同时写 gin.BodyBytesKey）。
func TestRepeatableBodyWorksWithShouldBindBodyWith(t *testing.T) {
	r := gin.New()
	r.Use(RepeatableBody())
	r.POST("/test", func(c *gin.Context) {
		var first, second struct {
			Name string `json:"name"`
		}
		// 绑两次，第二次靠的就是缓存。
		if err := c.ShouldBindBodyWithJSON(&first); err != nil {
			c.JSON(http.StatusOK, gin.H{"err": "first: " + err.Error()})
			return
		}
		if err := c.ShouldBindBodyWithJSON(&second); err != nil {
			c.JSON(http.StatusOK, gin.H{"err": "second: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"first": first.Name, "second": second.Name})
	})

	w := bodyPost(r, "application/json", `{"name":"李四"}`)
	if got := w.Body.String(); !strings.Contains(got, `"first":"李四"`) ||
		!strings.Contains(got, `"second":"李四"`) {
		t.Errorf("body = %s, 两次绑定都应拿到「李四」", got)
	}
}

// 只缓存配置里的 content-type，对齐 RepeatableFilter 只处理 JSON。
// 非 JSON 请求必须原样透传，绝不能去读它的 body。
func TestRepeatableBodySkipsNonJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantBuffer  bool
	}{
		{"纯 JSON", "application/json", true},
		// 实际请求多半带 charset，前缀匹配必须覆盖。
		{"JSON 带 charset", "application/json;charset=UTF-8", true},
		// content-type 大小写不敏感（RFC 9110）。
		{"JSON 大写", "APPLICATION/JSON", true},
		{"表单", "application/x-www-form-urlencoded", false},
		{"文件上传", "multipart/form-data; boundary=x", false},
		{"纯文本", "text/plain", false},
		{"无 content-type", "", false},
		// 前缀匹配会把 application/jsonp、application/json-patch+json
		// 也算作 JSON。这与 Java 侧 startsWithIgnoreCase 的行为一致，
		// 刻意不收紧成精确匹配：多缓存一个 JSON 族类型无害，
		// 而精确匹配会漏掉 PATCH 接口常用的 *+json 系列。
		{"jsonp 按前缀命中", "application/jsonp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffered bool
			var bodyReadable bool

			r := gin.New()
			r.Use(RepeatableBody())
			r.POST("/test", func(c *gin.Context) {
				buffered = BodyBytes(c) != nil
				// 没被缓存时，body 必须还是原封不动可读的。
				b, _ := io.ReadAll(c.Request.Body)
				bodyReadable = len(b) > 0
				c.Status(http.StatusOK)
			})

			bodyPost(r, tt.contentType, `{"name":"x"}`)

			if buffered != tt.wantBuffer {
				t.Errorf("缓存 = %v, 期望 %v", buffered, tt.wantBuffer)
			}
			// 两种情况都必须可读：缓存过的靠塞回去的 Reader，
			// 没缓存的靠原始 Body 未被触碰。
			if !bodyReadable {
				t.Error("handler 读不到 body（中间件把 body 吃掉了）")
			}
		})
	}
}

// 空 body 不设缓存键：缺键与空 body 对 gin 等价，
// 存个空切片只会让人误以为缓存过。
func TestRepeatableBodySkipsEmptyBody(t *testing.T) {
	var buffered bool
	r := gin.New()
	r.Use(RepeatableBody())
	r.POST("/test", func(c *gin.Context) {
		buffered = BodyBytes(c) != nil
		c.Status(http.StatusOK)
	})

	bodyPost(r, "application/json", "")

	if buffered {
		t.Error("空 body 不应设置缓存键")
	}
}

// 无 body 的 GET 请求不能 panic —— c.Request.Body 可能为 nil。
func TestRepeatableBodyHandlesNilBody(t *testing.T) {
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBody())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Body = nil
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}
}

// 带 JSON body 的 DELETE 是合法的，不能按方法排除
// （跳过 GET/DELETE 是 XssFilter 的行为，两者别搞混）。
func TestRepeatableBodyBuffersDelete(t *testing.T) {
	var buffered bool
	r := gin.New()
	r.Use(RepeatableBody())
	r.DELETE("/test", func(c *gin.Context) {
		buffered = BodyBytes(c) != nil
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", strings.NewReader(`{"ids":[1]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if !buffered {
		t.Error("带 JSON body 的 DELETE 也应缓存")
	}
}

// 超限必须拒绝，且走统一响应（HTTP 200 + body 里的业务码）。
// 关键是「拒绝」而非「截断」：截断会让 handler 拿到半截 JSON，
// 报出一个跟真实原因毫无关系的解析错误。
func TestRepeatableBodyRejectsOversized(t *testing.T) {
	const maxSize = 16
	cfg := config.RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  maxSize,
	}

	var handlerCalled bool
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBodyWithConfig(cfg))
	r.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// 用 chunked（ContentLength = -1）走「读满才发现超限」那条分支：
	// 有 ContentLength 时会被提前拒掉，测不到 LimitReader 的 +1 字节逻辑。
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{"name":"`+strings.Repeat("x", maxSize)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("超限请求不应进入 handler")
	}
	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200（业务码在响应体里）", w.Code)
	}

	var body response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v, body=%q", err, w.Body.String())
	}
	if body.Code != response.CodeFail {
		t.Errorf("code = %d, 期望 %d", body.Code, response.CodeFail)
	}
	// 具体尺寸只进日志，不能回给前端 —— 告诉调用方上限等于教它贴着上限打。
	if strings.Contains(body.Msg, "16") {
		t.Errorf("msg = %q, 不应泄露具体上限", body.Msg)
	}
}

// 提前拒绝：ContentLength 已经报超限就不该白读一遍。
func TestRepeatableBodyRejectsByContentLength(t *testing.T) {
	cfg := config.RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  8,
	}

	var handlerCalled bool
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBodyWithConfig(cfg))
	r.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	bodyPost(r, "application/json", `{"name":"很长很长的内容"}`)

	if handlerCalled {
		t.Error("ContentLength 超限的请求不应进入 handler")
	}
}

// 刚好等于上限必须放行 —— 边界不能差一。
func TestRepeatableBodyAllowsExactLimit(t *testing.T) {
	body := `{"name":"ab"}`
	cfg := config.RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  int64(len(body)),
	}

	var buffered bool
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBodyWithConfig(cfg))
	r.POST("/test", func(c *gin.Context) {
		buffered = BodyBytes(c) != nil
		c.Status(http.StatusOK)
	})

	// 同样走 chunked，确保命中读取后的长度判断。
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	r.ServeHTTP(httptest.NewRecorder(), req)

	if !buffered {
		t.Error("长度正好等于上限的请求应放行")
	}
}

// MaxBodySize <= 0 时回落默认值，不能变成「一律拒绝」。
func TestRepeatableBodyZeroMaxSizeFallsBack(t *testing.T) {
	var buffered bool
	r := gin.New()
	r.Use(RepeatableBodyWithConfig(config.RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
	}))
	r.POST("/test", func(c *gin.Context) {
		buffered = BodyBytes(c) != nil
		c.Status(http.StatusOK)
	})

	bodyPost(r, "application/json", `{"name":"x"}`)

	if !buffered {
		t.Error("MaxBodySize 为 0 应回落默认上限，而非拒绝所有请求")
	}
}

// 没挂中间件时 BodyBytes 返回 nil，调用方据此跳过打参数 ——
// 对齐 Java 侧 `if (request instanceof RepeatedlyRequestWrapper)`。
func TestBodyBytesWithoutMiddleware(t *testing.T) {
	var got []byte
	r := gin.New()
	r.POST("/test", func(c *gin.Context) {
		got = BodyBytes(c)
		c.Status(http.StatusOK)
	})

	bodyPost(r, "application/json", `{"name":"x"}`)

	if got != nil {
		t.Errorf("BodyBytes = %q, 未挂中间件时应返回 nil", got)
	}
}
