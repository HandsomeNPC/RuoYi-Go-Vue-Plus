package oplog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/response"
)

// capture 装一个同步 Recorder，返回取事件的函数。
// 生产实现是异步的，测试里同步收集才能确定性断言。
func capture(t *testing.T) func() *Event {
	t.Helper()

	var mu sync.Mutex
	var got *Event

	mu.Lock()
	prev := recorder
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		recorder = prev
		mu.Unlock()
	})

	Init(func(_ context.Context, evt *Event) {
		mu.Lock()
		defer mu.Unlock()
		got = evt
	})

	return func() *Event {
		mu.Lock()
		defer mu.Unlock()
		return got
	}
}

// newEngine 建一个挂了 oplog 的引擎。未挂 Recover：本包的 panic 用例要断言
// panic 确实继续往上抛，被 Recover 兜住就测不出来了。
func newEngine(handler gin.HandlerFunc, opts ...Option) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RepeatableBodyWithConfig(config.DefaultRepeatableBody()))
	r.POST("/client", Log("客户端管理", enum.BusinessTypeInsert, opts...), handler)
	r.DELETE("/client/:ids", Log("客户端管理", enum.BusinessTypeDelete, opts...), handler)
	r.POST("/client/export", Log("客户端管理", enum.BusinessTypeExport, opts...), handler)
	return r
}

// TestLogRecordsSuccess 成功请求记 status=0，并带上标题、业务类型、请求参数与响应体。
func TestLogRecordsSuccess(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	})

	req := httptest.NewRequest(http.MethodPost, "/client",
		strings.NewReader(`{"clientKey":"pc","clientSecret":"s"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.Title != "客户端管理" {
		t.Errorf("Title = %q, 期望 客户端管理", evt.Title)
	}
	if evt.BusinessType != enum.BusinessTypeInsert.Int() {
		t.Errorf("BusinessType = %d, 期望 %d", evt.BusinessType, enum.BusinessTypeInsert.Int())
	}
	if evt.OperatorType != enum.OperatorTypeManage.Int() {
		t.Errorf("OperatorType = %d, 期望后台用户 %d", evt.OperatorType, enum.OperatorTypeManage.Int())
	}
	if evt.Status != enum.BusinessStatusSuccess.Int() {
		t.Errorf("Status = %d, 期望正常 0", evt.Status)
	}
	if evt.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %q, 成功时应为空", evt.ErrorMsg)
	}
	if !strings.Contains(evt.OperParam, "clientKey") {
		t.Errorf("OperParam = %q, 应含 clientKey", evt.OperParam)
	}
	if !strings.Contains(evt.JSONResult, `"code"`) {
		t.Errorf("JSONResult = %q, 应含响应体", evt.JSONResult)
	}
	if evt.OperURL != "/client" || evt.RequestMethod != http.MethodPost {
		t.Errorf("OperURL/RequestMethod = %q/%q", evt.OperURL, evt.RequestMethod)
	}
	if !strings.HasSuffix(evt.Method, "()") {
		t.Errorf("Method = %q, 应以 () 结尾对齐 Java 形态", evt.Method)
	}
}

// TestLogRecordsBusinessFailure handler 走 c.Error 时记 status=1 并带错误文案。
// 本项目 handler 不自行渲染错误，这是业务失败的常规形态。
func TestLogRecordsBusinessFailure(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		_ = c.Error(errs.New(response.CodeFail, "客户端key已存在", "clientKey=pc"))
	})

	req := httptest.NewRequest(http.MethodPost, "/client", strings.NewReader(`{"clientKey":"pc"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.Status != enum.BusinessStatusFail.Int() {
		t.Errorf("Status = %d, 期望异常 1", evt.Status)
	}
	if !strings.Contains(evt.ErrorMsg, "客户端key已存在") {
		t.Errorf("ErrorMsg = %q, 应含业务提示", evt.ErrorMsg)
	}
	// Detail 只进服务端日志，但操作日志给管理员看，拼上便于定位。
	if !strings.Contains(evt.ErrorMsg, "clientKey=pc") {
		t.Errorf("ErrorMsg = %q, 应含 Detail 明细", evt.ErrorMsg)
	}
}

// TestLogRecordsPanic panic 时记一条失败日志，且 panic 继续往上抛
// （交给 middleware.Recover 渲染 500，不能被本包吞掉）。
func TestLogRecordsPanic(t *testing.T) {
	get := capture(t)
	r := newEngine(func(_ *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic 未继续向上抛，会绕过 middleware.Recover")
			}
		}()
		r.ServeHTTP(httptest.NewRecorder(), req)
	}()

	evt := get()
	if evt == nil {
		t.Fatal("panic 时未记录操作日志")
	}
	if evt.Status != enum.BusinessStatusFail.Int() {
		t.Errorf("Status = %d, 期望异常 1", evt.Status)
	}
	if evt.ErrorMsg != msgPanic {
		t.Errorf("ErrorMsg = %q, 期望 %q", evt.ErrorMsg, msgPanic)
	}
}

// TestLogRecordsPathParams 路径参数须入库：Java 侧 joinPoint.getArgs() 含
// @PathVariable，删了哪几个客户端是审计最需要的信息。
func TestLogRecordsPathParams(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	})

	req := httptest.NewRequest(http.MethodDelete,
		"/client/1762000000000000001,1762000000000000002", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if !strings.Contains(evt.OperParam, "1762000000000000001") {
		t.Errorf("OperParam = %q, 应含被删主键", evt.OperParam)
	}
	if evt.BusinessType != enum.BusinessTypeDelete.Int() {
		t.Errorf("BusinessType = %d, 期望删除 %d", evt.BusinessType, enum.BusinessTypeDelete.Int())
	}
}

// TestLogExcludesSensitiveParams 密码类字段不得落库。
func TestLogExcludesSensitiveParams(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	})

	req := httptest.NewRequest(http.MethodPost, "/client",
		strings.NewReader(`{"clientKey":"pc","password":"admin123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if strings.Contains(evt.OperParam, "admin123") || strings.Contains(evt.OperParam, "password") {
		t.Errorf("OperParam = %q, 不应含密码字段", evt.OperParam)
	}
	if !strings.Contains(evt.OperParam, "clientKey") {
		t.Errorf("OperParam = %q, 非敏感字段应保留", evt.OperParam)
	}
}

// TestLogSkipsBinaryResponse 导出接口响应体是 xlsx 二进制，
// 不该落进 json_result（对照 Java export 返回 void，jsonResult 为 null）。
func TestLogSkipsBinaryResponse(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.Header("Content-Type",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = c.Writer.Write([]byte("PK\x03\x04binary-xlsx-bytes"))
	})

	req := httptest.NewRequest(http.MethodPost, "/client/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.JSONResult != "" {
		t.Errorf("JSONResult = %q, 二进制响应不应落库", evt.JSONResult)
	}
	// 抄一份不能妨碍真实响应下发。
	if !strings.Contains(w.Body.String(), "binary-xlsx-bytes") {
		t.Errorf("响应体被截留: %q", w.Body.String())
	}
	if evt.BusinessType != enum.BusinessTypeExport.Int() {
		t.Errorf("BusinessType = %d, 期望导出 %d", evt.BusinessType, enum.BusinessTypeExport.Int())
	}
}

// TestLogPassesThroughResponse 挂了本中间件后响应体须与不挂时逐字节一致。
func TestLogPassesThroughResponse(t *testing.T) {
	_ = capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok("payload"))
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "payload") {
		t.Errorf("响应体 = %q, 应原样透传", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}
}

// TestLogWithoutRequestData WithoutRequestData / WithoutResponseData 关掉对应采集。
func TestLogWithoutRequestData(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	}, WithoutRequestData(), WithoutResponseData())

	req := httptest.NewRequest(http.MethodPost, "/client", strings.NewReader(`{"clientKey":"pc"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.OperParam != "" {
		t.Errorf("OperParam = %q, 已关闭应为空", evt.OperParam)
	}
	if evt.JSONResult != "" {
		t.Errorf("JSONResult = %q, 已关闭应为空", evt.JSONResult)
	}
}

// TestLogWithExcludeParams 追加的排除字段不落库。
func TestLogWithExcludeParams(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	}, WithExcludeParams("clientSecret"))

	req := httptest.NewRequest(http.MethodPost, "/client",
		strings.NewReader(`{"clientKey":"pc","clientSecret":"top-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if strings.Contains(evt.OperParam, "top-secret") {
		t.Errorf("OperParam = %q, 不应含被排除字段", evt.OperParam)
	}
}

// TestLogWithoutRecorder 未注册 Recorder 时请求照常完成，不 panic
// ——日志缺失不该让业务整体失败。
func TestLogWithoutRecorder(t *testing.T) {
	mu.Lock()
	prev := recorder
	recorder = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		recorder = prev
		mu.Unlock()
	})

	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}
}

// TestLogRecordsNoLoginUser 未登录时用户字段留零，不 panic
// （对照 Java loginUser 为 null 的分支）。
func TestLogRecordsNoLoginUser(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OkVoid())
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.UserID != 0 || evt.OperName != "" {
		t.Errorf("未登录时 UserID/OperName = %d/%q, 应留零", evt.UserID, evt.OperName)
	}
}

// TestLogTruncatesLongContent 超长响应体截断到落库上限，不能撑爆 varchar(4000)。
func TestLogTruncatesLongContent(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok(strings.Repeat("x", 20000)))
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if n := len([]rune(evt.JSONResult)); n > maxContentLength {
		t.Errorf("JSONResult 字符数 = %d, 超出上限 %d", n, maxContentLength)
	}
}

// TestLogRecordsFailureFromBodyCode handler 直接 c.JSON(200, response.Fail(...))
// 而不登记 c.Error 时，也要判为失败——否则会记出 status=0 却
// json_result 带 code:500 的自相矛盾日志，且与 repeatsubmit 的判定相反。
func TestLogRecordsFailureFromBodyCode(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Fail("客户端不存在"))
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.Status != enum.BusinessStatusFail.Int() {
		t.Errorf("Status = %d, 期望异常 1（响应体 code 非 200）", evt.Status)
	}
	if !strings.Contains(evt.ErrorMsg, "客户端不存在") {
		t.Errorf("ErrorMsg = %q, 应取响应体 msg", evt.ErrorMsg)
	}
}

// TestLogRecordsFailureFromHTTPStatus 非 2xx 状态码判为失败。
func TestLogRecordsFailureFromHTTPStatus(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	})

	req := httptest.NewRequest(http.MethodPost, "/client", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.Status != enum.BusinessStatusFail.Int() {
		t.Errorf("Status = %d, 期望异常 1（HTTP 403）", evt.Status)
	}
	if !strings.Contains(evt.ErrorMsg, "403") {
		t.Errorf("ErrorMsg = %q, 应含状态码", evt.ErrorMsg)
	}
}

// TestLogBinaryResponseCountsAsSuccess 二进制响应体解不出 code，
// 不能因此判为失败——导出成功必须记 status=0。
func TestLogBinaryResponseCountsAsSuccess(t *testing.T) {
	get := capture(t)
	r := newEngine(func(c *gin.Context) {
		c.Header("Content-Type",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = c.Writer.Write([]byte("PK\x03\x04xlsx"))
	})

	req := httptest.NewRequest(http.MethodPost, "/client/export", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	evt := get()
	if evt == nil {
		t.Fatal("未记录操作日志")
	}
	if evt.Status != enum.BusinessStatusSuccess.Int() {
		t.Errorf("Status = %d, 导出成功应记正常 0", evt.Status)
	}
	if evt.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %q, 应为空", evt.ErrorMsg)
	}
}

// TestLimitCutsOnRuneBoundary 截断按 rune 而非 byte：
// 中文按字节切会把最后一个字切成乱码。
func TestLimitCutsOnRuneBoundary(t *testing.T) {
	in := strings.Repeat("客户端管理", 10) // 每字 3 字节
	got := limit(in, 7)

	if n := len([]rune(got)); n != 7 {
		t.Errorf("limit 后字符数 = %d, 期望 7", n)
	}
	if !utf8ValidString(got) {
		t.Errorf("limit(%q) = %q, 含非法 UTF-8 序列", in, got)
	}
}

// utf8ValidString 判断字符串是否为合法 UTF-8。
func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
