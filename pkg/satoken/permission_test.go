package satoken

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/response"
)

// newGuardedEngine 装配一个只挂目标中间件的引擎，用 called 回执断言是否放行到下游。
// 不挂 TokenInterceptor，以此模拟请求未携带 token。
func newGuardedEngine(guard gin.HandlerFunc, called *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/probe", guard, func(c *gin.Context) {
		*called = true
		c.JSON(http.StatusOK, response.OkVoid())
	})
	return r
}

// decodeR 解出统一响应的 code/msg。
func decodeR(t *testing.T, body []byte) (int, string) {
	t.Helper()
	var got struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, body)
	}
	return got.Code, got.Msg
}

// TestCheckPermissionRejectsWithoutToken 无 token 必须 401 且不放行下游。
// 走 token 空分支，不触碰 Redis/Manager，故无需初始化 sa-token。
func TestCheckPermissionRejectsWithoutToken(t *testing.T) {
	var called bool
	r := newGuardedEngine(CheckPermission("system:client:list"), &called)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	// HTTP 状态恒 200，业务码才承载失败语义。
	if w.Code != http.StatusOK {
		t.Errorf("HTTP 状态 = %d, 期望 200", w.Code)
	}
	code, msg := decodeR(t, w.Body.Bytes())
	if code != response.CodeUnauthorized {
		t.Errorf("code = %d, 期望 %d", code, response.CodeUnauthorized)
	}
	if msg != msgNotLogin {
		t.Errorf("msg = %q, 期望 %q", msg, msgNotLogin)
	}
	if called {
		t.Error("未登录时不应放行到下游 handler")
	}
}

// TestCheckRoleRejectsWithoutToken CheckRole 与 CheckPermission 共用 check，行为应一致。
func TestCheckRoleRejectsWithoutToken(t *testing.T) {
	var called bool
	r := newGuardedEngine(CheckRole("superadmin"), &called)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	code, msg := decodeR(t, w.Body.Bytes())
	if code != response.CodeUnauthorized {
		t.Errorf("code = %d, 期望 %d", code, response.CodeUnauthorized)
	}
	if msg != msgNotLogin {
		t.Errorf("msg = %q, 期望 %q", msg, msgNotLogin)
	}
	if called {
		t.Error("未登录时不应放行到下游 handler")
	}
}
