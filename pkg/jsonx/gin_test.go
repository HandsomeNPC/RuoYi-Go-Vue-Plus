package jsonx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/response"
)

// 本文件用外部测试包（jsonx_test）经 gin 的真实 c.JSON / ShouldBindJSON 验证收口，
// 而 jsonx_test.go 只测 codec 本身。两者互补：codec 对了但没接到 gin 上等于没修。

type payload struct {
	ID      int64   `json:"id"`
	RoleIDs []int64 `json:"roleIds"`
	Timeout int64   `json:"timeout"`
}

// newEngine 装配一个回显 payload 的引擎，模拟真实 handler 的绑定+响应两段。
func newEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	jsonx.Init()

	r := gin.New()
	r.POST("/echo", func(c *gin.Context) {
		var in payload
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusOK, response.Fail(err.Error()))
			return
		}
		c.JSON(http.StatusOK, response.Ok(in))
	})
	return r
}

func post(t *testing.T, body string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newEngine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, 期望 200", w.Code)
	}
	return w.Body.String()
}

// TestGinRoundTripStringID 前端把出参里的字符串 id 原样回传，须能绑定并再次以字符串出参。
// 这是编辑场景的完整闭环，也是本次改造的核心诉求。
func TestGinRoundTripStringID(t *testing.T) {
	got := post(t, `{"id":"1762000000000000001","roleIds":["1761300000000000001"],"timeout":604800}`)

	for _, want := range []string{
		`"id":"1762000000000000001"`,        // 雪花 id 出参仍是字符串
		`"roleIds":["1761300000000000001"]`, // 切片元素同样
		`"timeout":604800`,                  // 普通数值保持数字
	} {
		if !strings.Contains(got, want) {
			t.Errorf("响应 = %s\n应包含 %s", got, want)
		}
	}
}

// TestGinAcceptsNumericID 老前端/第三方仍可能送数字形态，不能因改造而拒收。
func TestGinAcceptsNumericID(t *testing.T) {
	got := post(t, `{"id":1762000000000000001,"roleIds":[1761300000000000001]}`)

	if !strings.Contains(got, `"id":"1762000000000000001"`) {
		t.Errorf("响应 = %s\n数字入参也应正确解析", got)
	}
	if !strings.Contains(got, `"roleIds":["1761300000000000001"]`) {
		t.Errorf("响应 = %s\n数字数组也应正确解析", got)
	}
}

// TestGinZeroStaysNumber parentId/avatar 这类「根节点为 0」的字段必须仍是数字：
// 前端 dept/index.vue 按 `!== 0` 严格比较判断有无上级，字符串 "0" 会让它误判。
func TestGinZeroStaysNumber(t *testing.T) {
	got := post(t, `{"id":0}`)

	if !strings.Contains(got, `"id":0`) {
		t.Errorf("响应 = %s\n零值须保持数字形态", got)
	}
	if strings.Contains(got, `"id":"0"`) {
		t.Errorf("响应 = %s\n零值不可转成字符串", got)
	}
}

// TestGinBindErrorReadable 绑定失败的错误文案要能指出问题，不能因换 codec 变成天书。
// handler 会把它塞进 response.Fail 回前端，可读性直接影响排障。
func TestGinBindErrorReadable(t *testing.T) {
	got := post(t, `{"id":"not-a-number"}`)

	if !strings.Contains(got, `"code":500`) {
		t.Errorf("响应 = %s\n非法值应返回失败", got)
	}
	if !strings.Contains(got, "not-a-number") {
		t.Errorf("响应 = %s\n错误文案应含原值便于定位", got)
	}
}
