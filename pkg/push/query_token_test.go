package push

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// TestNormalizeQueryTokenStripsBearer 复现前端实际请求形态：
// ?Authorization=Bearer <jwt>&clientid=xxx —— 前缀必须被剥掉。
//
// EventSource/WebSocket 不能自定义请求头，token 只能走 query；而 sa-token-go
// 的 query 分支不调 extractBearerToken，未规范化时会拿 "Bearer eyJ..." 整串
// 去查会话，必然 401。
func TestNormalizeQueryTokenStripsBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")

	const jwt = "eyJhbGciOiJIUzI1NiJ9.payload.sig"
	var got, gotClientID string

	r := gin.New()
	r.GET("/resource/message", NormalizeQueryToken(), func(c *gin.Context) {
		got = c.Query("Authorization")
		gotClientID = c.Query("clientid")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/resource/message?Authorization=Bearer%20"+jwt+"&clientid=e5cd7e4891bf95d1", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got != jwt {
		t.Errorf("token = %q, 期望剥掉 Bearer 前缀后的 %q", got, jwt)
	}
	// 重写 RawQuery 不能顺手弄丢其它参数。
	if gotClientID != "e5cd7e4891bf95d1" {
		t.Errorf("clientid 被弄丢了: %q", gotClientID)
	}
}

// TestNormalizeQueryTokenCaseInsensitive 前缀大小写不敏感（对齐 header 侧的 EqualFold）。
func TestNormalizeQueryTokenCaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")

	for _, prefix := range []string{"Bearer%20", "bearer%20", "BEARER%20"} {
		var got string
		r := gin.New()
		r.GET("/p", NormalizeQueryToken(), func(c *gin.Context) {
			got = c.Query("Authorization")
		})
		r.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/p?Authorization="+prefix+"tok", nil))

		if got != "tok" {
			t.Errorf("前缀 %q: token = %q, 期望 %q", prefix, got, "tok")
		}
	}
}

// TestNormalizeQueryTokenLeavesOthersAlone 不带前缀的 token 与无 query 的请求都不该被改动。
func TestNormalizeQueryTokenLeavesOthersAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")

	tests := []struct {
		name, query, want string
	}{
		// 裸 token（header 形态已被 sa-token 处理过）原样通过。
		{"裸 token", "?Authorization=rawtoken", "rawtoken"},
		{"无 token", "?clientid=x", ""},
		{"无 query", "", ""},
		// "bearerX" 不是前缀（缺空格），不能误剥。
		{"形似前缀不误剥", "?Authorization=bearerX", "bearerX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			r := gin.New()
			r.GET("/p", NormalizeQueryToken(), func(c *gin.Context) {
				got = c.Query("Authorization")
			})
			r.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, "/p"+tt.query, nil))

			if got != tt.want {
				t.Errorf("token = %q, 期望 %q", got, tt.want)
			}
		})
	}
}
