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

// TestRepeatableBodyAllowsRebind 中间件读完 body 后 handler 必须还能绑到参数。
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

// TestRepeatableBodyWorksWithShouldBindBodyWith 能复用缓存反复绑定不同结构体。
func TestRepeatableBodyWorksWithShouldBindBodyWith(t *testing.T) {
	r := gin.New()
	r.Use(RepeatableBody())
	r.POST("/test", func(c *gin.Context) {
		var first, second struct {
			Name string `json:"name"`
		}
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

// TestRepeatableBodySkipsNonJSON 只缓存配置里的 content-type，非 JSON 请求原样透传。
func TestRepeatableBodySkipsNonJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantBuffer  bool
	}{
		{"纯 JSON", "application/json", true},
		{"JSON 带 charset", "application/json;charset=UTF-8", true},
		{"JSON 大写", "APPLICATION/JSON", true},
		{"表单", "application/x-www-form-urlencoded", false},
		{"文件上传", "multipart/form-data; boundary=x", false},
		{"纯文本", "text/plain", false},
		{"无 content-type", "", false},
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
				b, _ := io.ReadAll(c.Request.Body)
				bodyReadable = len(b) > 0
				c.Status(http.StatusOK)
			})

			bodyPost(r, tt.contentType, `{"name":"x"}`)

			if buffered != tt.wantBuffer {
				t.Errorf("缓存 = %v, 期望 %v", buffered, tt.wantBuffer)
			}
			if !bodyReadable {
				t.Error("handler 读不到 body（中间件把 body 吃掉了）")
			}
		})
	}
}

// TestRepeatableBodySkipsEmptyBody 空 body 不设缓存键。
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

// TestRepeatableBodyHandlesNilBody 无 body 的 GET 请求不能 panic。
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

// TestRepeatableBodyBuffersDelete 带 JSON body 的 DELETE 也应缓存。
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

// TestRepeatableBodyRejectsOversized 超限必须拒绝，且走统一响应。
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

	// chunked 走「读满才发现超限」分支，避开 ContentLength 提前拒掉。
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
	if strings.Contains(body.Msg, "16") {
		t.Errorf("msg = %q, 不应泄露具体上限", body.Msg)
	}
}

// TestRepeatableBodyRejectsByContentLength ContentLength 已报超限就不该白读一遍。
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

// TestRepeatableBodyAllowsExactLimit 刚好等于上限必须放行。
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

// TestRepeatableBodyZeroMaxSizeFallsBack MaxBodySize <= 0 时回落默认值。
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

// TestBodyBytesWithoutMiddleware 没挂中间件时 BodyBytes 返回 nil。
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
