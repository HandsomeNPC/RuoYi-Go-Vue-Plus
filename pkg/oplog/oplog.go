// Package oplog 提供操作日志注解（装饰器）。
//
// 落库实现不在本包：pkg 不依赖 internal 的 service/repository，故这里只组装
// Event，由 internal/system 在进程启动时经 Init 反向注册 Recorder 消费。
// 落库由 internal/system 在启动时反向注册 Recorder 消费 Event，与业务包解耦。
//
// 与原实现的差异：
//   - Method 取 gin 的 HandlerName（无 AOP 切点，拿不到"类名.方法名"）；
//   - OperParam 在 handler 之前采集而非之后——原实现从线程绑定的 request 取，
//     Go 的 body 是一次性流，handler 消费后再读就晚了；
//   - 响应体边写边抄一份（不拦截后补发），故 panic 时半截响应的行为与不挂本
//     中间件时一致，无需 repeatsubmit 那样规避 Written() 与 Recover 的冲突。
package oplog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// 各字段的落库长度上限，与 sys_oper_log 列宽对应。maxNameLength/maxIPLength 是补的：严格模式下超长会让整条日志插入失败。
const (
	maxURLLength       = 255 // oper_url varchar(255)
	maxClientKeyLength = 32  // client_key / device_type varchar(32)
	maxContentLength   = 3800
	maxNameLength      = 50  // oper_name / dept_name / browser / os varchar(50)
	maxIPLength        = 128 // oper_ip varchar(128)
)

// msgPanic panic 展开时写入 ErrorMsg 的占位文案。
//
// 不 recover 取真实 panic 值：recover 后 re-panic 会把 middleware.Recover 那边
// debug.Stack() 的栈顶挪到本包，排障时反而看不到出事的行。真实 panic 值与完整栈
// 由 Recover 记录，这里只需标记本次操作失败。
const msgPanic = "系统异常"

// Event 操作日志事件。
// OperLocation 留空由 Recorder 侧补（反查 IP 归属地要走地址库，不该占住请求线程）。
type Event struct {
	Title         string
	BusinessType  int
	Method        string
	RequestMethod string
	OperatorType  int
	OperName      string
	UserID        int64
	DeptID        int64
	DeptName      string
	ClientKey     string
	DeviceType    string
	Browser       string
	OS            string
	OperURL       string
	OperIP        string
	OperParam     string
	JSONResult    string
	Status        int
	ErrorMsg      string
	CostTime      int64
}

// Recorder 消费操作日志事件，由 internal/system 注册。
// 实现方负责异步落库并自行兜住错误——本包调用它时不等待、不检查返回。
type Recorder func(ctx context.Context, evt *Event)

var (
	mu       sync.RWMutex
	recorder Recorder
)

// Init 注册落库实现，须在 database.Init 之后调用（Recorder 内部要写库）。
func Init(r Recorder) {
	if r == nil {
		panic("oplog: Recorder 不能为空")
	}
	mu.Lock()
	recorder = r
	mu.Unlock()
}

// get 取已注册的 Recorder，未注册返回 nil。
//
// 与 repeatsubmit.get 不同，这里不 panic：日志缺失不该让业务请求整体失败，
// 漏注册由启动日志与下面的告警暴露。
func get() Recorder {
	mu.RLock()
	r := recorder
	mu.RUnlock()
	return r
}

// options @Log 注解的可选项。
type options struct {
	operatorType     enum.OperatorType
	saveRequestData  bool
	saveResponseData bool
	excludeParams    []string
}

// Option 配置项。
type Option func(*options)

// WithOperatorType 指定操作人类别，默认 OperatorTypeManage。
func WithOperatorType(t enum.OperatorType) Option {
	return func(o *options) { o.operatorType = t }
}

// WithoutRequestData 不记录请求参数。
func WithoutRequestData() Option {
	return func(o *options) { o.saveRequestData = false }
}

// WithoutResponseData 不记录响应参数。
func WithoutResponseData() Option {
	return func(o *options) { o.saveResponseData = false }
}

// WithExcludeParams 追加不记录的请求参数名。
// constant.ExcludeProperties 里的密码类字段无需在此重复。
func WithExcludeParams(names ...string) Option {
	return func(o *options) { o.excludeParams = append(o.excludeParams, names...) }
}

// Log 操作日志注解。title 为模块名，businessType 为业务类型。
//
// 须排在鉴权之后：被拒的请求在鉴权阶段就返回，进不到本中间件。
func Log(title string, businessType enum.BusinessType, opts ...Option) gin.HandlerFunc {
	o := &options{
		operatorType:     enum.OperatorTypeManage,
		saveRequestData:  true,
		saveResponseData: true,
	}
	for _, opt := range opts {
		opt(o)
	}

	return func(c *gin.Context) {
		run(c, title, businessType, o)
	}
}

// run 执行一次操作日志记录。
func run(c *gin.Context, title string, businessType enum.BusinessType, o *options) {
	evt := &Event{
		Title:         title,
		BusinessType:  businessType.Int(),
		OperatorType:  o.operatorType.Int(),
		Method:        handlerName(c),
		RequestMethod: c.Request.Method,
		OperURL:       limit(c.Request.URL.Path, maxURLLength),
		// X-Forwarded-For 可伪造且不校验格式，长度必须自己兜。
		OperIP:    limit(ip.ClientIP(c.Request), maxIPLength),
		ClientKey: limit(c.GetHeader(constant.ClientIDHeader), maxClientKeyLength),
		Status:    enum.BusinessStatusSuccess.Int(),
	}
	fillLoginUser(c, evt)

	// 入参在 handler 之前采集：body 是一次性流，handler 绑定后再读就没了。
	if o.saveRequestData {
		evt.OperParam = requestParam(c, o.excludeParams)
	}

	// 边写边抄：不缓冲后补发，避免 panic 时抄本与真实响应不一致。
	original := c.Writer
	capture := &responseCapture{ResponseWriter: original}
	if o.saveResponseData {
		c.Writer = capture
	}

	start := time.Now()
	completed := false
	defer func() {
		c.Writer = original
		evt.CostTime = time.Since(start).Milliseconds()

		if !completed {
			// panic 正在展开：记失败后让它继续往上抛，不 recover。
			evt.Status = enum.BusinessStatusFail.Int()
			evt.ErrorMsg = msgPanic
			emit(c, evt)
			return
		}

		// 响应体先取出：既要落 json_result，也是 failed 判定业务 code 的依据。
		// 未开启响应采集时为空串，failed 退化为只看 c.Errors 与 HTTP 状态。
		body := capture.text()
		if o.saveResponseData {
			evt.JSONResult = limit(body, maxContentLength)
		}
		if fail, reason := failed(c, body); fail {
			evt.Status = enum.BusinessStatusFail.Int()
			evt.ErrorMsg = limit(reason, maxContentLength)
		}
		emit(c, evt)
	}()

	c.Next()
	completed = true
}

// emit 把事件交给 Recorder。
//
// ctx 用 WithoutCancel：Recorder 侧是异步落库，响应一发完请求 ctx 即取消，
// 不脱开生命周期落库必失败（与 LoginInfoSvcApp.RecordLoginInfo 同一处理）。
func emit(c *gin.Context, evt *Event) {
	r := get()
	if r == nil {
		log.Printf("[oplog] 未注册 Recorder,操作日志丢弃: %s %s", evt.RequestMethod, evt.OperURL)
		return
	}
	r(context.WithoutCancel(c.Request.Context()), evt)
}

// fillLoginUser 填充登录用户相关字段，未登录时整体留零。
//
// 逐个截断：这些列是 varchar(32)/(50)，而会话与请求头的内容不受本进程控制
// （oper_ip 取自可伪造的 X-Forwarded-For），超长会让异步落库整条失败——
// 那时只剩一行错误日志，操作记录静默丢失。
func fillLoginUser(c *gin.Context, evt *Event) {
	lu := loginhelper.GetLoginUser(c)
	if lu == nil {
		return
	}
	evt.OperName = limit(lu.Username, maxNameLength)
	evt.UserID = lu.UserID
	evt.DeptID = lu.DeptID
	evt.DeptName = limit(lu.DeptName, maxNameLength)
	evt.DeviceType = limit(lu.DeviceType, maxClientKeyLength)
	evt.Browser = limit(lu.Browser, maxNameLength)
	evt.OS = limit(lu.OS, maxNameLength)
	// 请求头没带 clientid 时回落会话里的。
	if evt.ClientKey == "" {
		evt.ClientKey = limit(lu.ClientKey, maxClientKeyLength)
	}
}

// requestParam 采集入参。
//
// query/form 有值就用它，否则（PUT/POST/DELETE）取方法入参。
// 原实现的「方法入参」含 @RequestBody 也含 @PathVariable，
// 故除请求体外还要带上路径参数——否则 DELETE /client/{ids} 只剩一条
// "删了点什么" 的空记录，而删了哪几个客户端恰是审计最需要的信息。
func requestParam(c *gin.Context, exclude []string) string {
	if params := middleware.SanitizeFormParam(c, maxContentLength, exclude); params != "" {
		return params
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		return joinParams(
			pathParam(c, exclude),
			middleware.SanitizeJSONParam(middleware.BodyBytes(c), maxContentLength, exclude),
		)
	default:
		return ""
	}
}

// pathParam 把路径参数序列化成 JSON 串，无参数返回空串。
func pathParam(c *gin.Context, exclude []string) string {
	if len(c.Params) == 0 {
		return ""
	}
	params := make(map[string]string, len(c.Params))
	for _, p := range c.Params {
		if !excluded(p.Key, exclude) {
			params[p.Key] = p.Value
		}
	}
	if len(params) == 0 {
		return ""
	}
	out, err := jsonx.Marshal(params)
	if err != nil {
		return ""
	}
	return string(out)
}

// excluded 判断参数名是否属于不记录的字段，比对不区分大小写。
func excluded(name string, extra []string) bool {
	for _, field := range constant.ExcludeProperties {
		if strings.EqualFold(name, field) {
			return true
		}
	}
	for _, field := range extra {
		if strings.EqualFold(name, field) {
			return true
		}
	}
	return false
}

// joinParams 用空格拼接非空片段。
func joinParams(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return limit(strings.Join(kept, " "), maxContentLength)
}

// lastError 取 handler 登记的最后一个错误，无错误返回 nil。
//
// 本项目 handler 不直接渲染错误而是 c.Error 后交给 middleware.Recover，
// 故业务失败与直接抛异常在此等价——都判为 FAIL。
func lastError(c *gin.Context) error {
	if len(c.Errors) == 0 {
		return nil
	}
	return c.Errors.Last().Err
}

// failed 判定本次请求是否失败，与 repeatsubmit.succeeded 用同一组信号：
// c.Error 非空、HTTP 非 2xx、响应体业务 code 非 200 都算失败。
//
// 只看 c.Errors 是不够的：handler 也可以直接 c.JSON(200, response.Fail(...))
// 而不登记错误（user_handler 就是这么写的），那样会记出一条 status=0 却
// json_result 带 code:500 的自相矛盾的日志。原实现只认异常，
// 但本项目多出这条 code 通路，且与 repeatsubmit 判定不一致会让同一次请求
// 在两个中间件里得出相反结论。
//
// 返回 reason 供 ErrorMsg 使用，body 非 R 结构（文件流等）时按成功处理。
func failed(c *gin.Context, body string) (bool, string) {
	if err := lastError(c); err != nil {
		return true, errorMsg(err)
	}
	if status := c.Writer.Status(); status < 200 || status > 299 {
		return true, fmt.Sprintf("HTTP %d", status)
	}
	code, msg, ok := parseResult(body)
	if !ok || code == response.CodeSuccess {
		return false, ""
	}
	if msg == "" {
		msg = fmt.Sprintf("业务码 %d", code)
	}
	return true, msg
}

// parseResult 从响应体取业务 code 与 msg，ok=false 表示不是 R 结构。
func parseResult(body string) (int, string, bool) {
	if body == "" {
		return 0, "", false
	}
	var r struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := jsonx.Unmarshal([]byte(body), &r); err != nil || r.Code == nil {
		return 0, "", false
	}
	return *r.Code, r.Msg, true
}

// errorMsg 取错误文案。ServiceError 的 Detail 只进服务端日志，
// 操作日志给管理员看，故拼上便于定位。
func errorMsg(err error) string {
	var se *errs.ServiceError
	if errors.As(err, &se) && se.Detail != "" {
		return se.Msg + " | " + se.Detail
	}
	return err.Error()
}

// handlerName 取 handler 函数名作为 Method，补 "()"。
// gin 的 HandlerName 形如 ".../internal/system/handler.(*ClientApi).Add-fm"，
// "-fm" 是方法值的编译期后缀，去掉。
func handlerName(c *gin.Context) string {
	name := strings.TrimSuffix(c.HandlerName(), "-fm")
	if name == "" {
		return ""
	}
	return limit(name+"()", maxContentLength)
}

// limit 按字符数截断。
// 按 rune 而非 byte 截：中文文案按字节切会把最后一个字切成乱码。
func limit(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s // 一个 rune 至少 1 字节，字节数没超字符数必没超。
	}
	count := 0
	for i := range s {
		if count == maxLen {
			return s[:i]
		}
		count++
	}
	return s
}

// responseCapture 抄一份响应体的 gin.ResponseWriter 包装。
// 写入原样透传，另抄一份供落库；抄本封顶，导出接口的 xlsx 不该整个堆在内存里。
type responseCapture struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

// maxCaptureBytes 抄本上限。落库要截到 maxContentLength 个字符，
// 按 UTF-8 单字符最多 4 字节留够余量即可。
const maxCaptureBytes = maxContentLength * 4

func (w *responseCapture) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseCapture) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// capture 把响应字节抄进缓冲区，超过上限的部分丢弃。
//
// 先看 Content-Type：非 JSON 响应（导出的 xlsx）最终会被 text 丢掉，
// 白抄一遍等于给每次导出多分配一份 15KB。头在首次 Write 前已写好，这里判得准。
func (w *responseCapture) capture(b []byte) {
	if !w.jsonBody() {
		return
	}
	if room := maxCaptureBytes - w.buf.Len(); room > 0 {
		w.buf.Write(b[:min(len(b), room)])
	}
}

// jsonBody 判断响应是否 JSON。
func (w *responseCapture) jsonBody() bool {
	return strings.HasPrefix(
		strings.ToLower(w.Header().Get("Content-Type")), middleware.ContentTypeJSON)
}

// text 返回抄本。非 JSON 响应返回空串：导出接口的响应体是 xlsx 二进制，
// 落进 json_result 既无可读性又白占 4000 字节。
func (w *responseCapture) text() string {
	if !w.jsonBody() {
		return ""
	}
	return w.buf.String()
}
