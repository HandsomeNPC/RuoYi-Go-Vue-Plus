package resource

import (
	"net/http"
	"time"

	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"
	"github.com/gin-gonic/gin"

	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
	"ruoyi-go-vue-plus/internal/resource/handler"
	"ruoyi-go-vue-plus/internal/resource/push"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/ratelimiter"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/audit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/checklogin"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/notfound"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/operlog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/permission"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/recover"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
)

const ossLogTitle = "OSS对象存储"

const ossConfigLogTitle = "对象存储配置"

const ossConfigStatusLogTitle = "对象存储状态修改"

// RegisterRoutes 注册 resource 路由。
//
// 内部路由 /oss、/message 不带模块名前缀，由调用方传 prefix：
//   - modular（InitRouter）传 ""，前端打 /resource/oss，由 nginx 剥前缀后转 /oss；
//   - standalone 传 "/resource"，无网关，进程内自带完整路径。
//
// 推送端点同理（见 RegisterPushRoutes），同样靠 prefix 拼，故 nginx 对 /resource/ 一视同仁地剥前缀即可。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	// 公开路由(免鉴权)：探针，与 system/auth/monitor 对齐。
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok(gin.H{"module": "resource", "message": "pong"}))
	})

	protected := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	protected.Use(plugin.TokenInterceptor(), audit.AuditContext())

	// 不校验权限码，仅需登录：消息盒子只返回当前用户自己的消息。
	protected.GET("/message/box", checklogin.CheckLogin(), handler.MessageApiApp.GetBox)

	// 短信/邮箱验证码：登录前就要调，必须免鉴权。这两个端点落在带
	// TokenInterceptor 的组内，故显式 Ignore 放行。
	//
	// 限流：同一手机号/邮箱 60 秒 1 次。Go 侧限流是 handler 之前的
	// 中间件，做不到按开关关闭拆分——后果仅是邮箱功能关闭时重复请求也
	// 占额度，接口本就返回失败，不值得为此把限流下沉进 handler。
	protected.GET("/sms/code", sagin.Ignore(),
		ratelimiter.RateLimiterWithKeyFunc(time.Minute, 1,
			func(c *gin.Context) string { return c.Query("phoneNumber") }, 0, ""),
		handler.CaptchaApiApp.SmsCode)
	protected.GET("/email/code", sagin.Ignore(),
		ratelimiter.RateLimiterWithKeyFunc(time.Minute, 1,
			func(c *gin.Context) string { return c.Query("email") }, 0, ""),
		handler.CaptchaApiApp.EmailCode)

	// /oss/config 必须先于 /oss 注册组：两组的静态段虽然 gin 能区分，
	// 但先注册更具体的路径可避免 DELETE /oss/config/:ids 被 /oss/:ossIds 吃掉。
	ossConfig := protected.Group("/oss/config")
	// 详情页复用 list 权限码而非 query。
	ossConfig.GET("/list", permission.CheckPermission("system:ossConfig:list"),
		handler.OssConfigApiApp.List)
	ossConfig.GET("/:ossConfigId", permission.CheckPermission("system:ossConfig:list"),
		handler.OssConfigApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /oss/config/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，仍记一条失败日志。
	ossConfig.POST("", permission.CheckPermission("system:ossConfig:add"),
		operlog.Log(ossConfigLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.Add)
	ossConfig.PUT("", permission.CheckPermission("system:ossConfig:edit"),
		operlog.Log(ossConfigLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.Edit)
	// changeStatus 路径更具体，须注册在 PUT "" 之后。挂防重：
	// 它会把全表刷成非默认再置目标行，重复提交期间存在"一个默认都没有"的窗口。
	ossConfig.PUT("/changeStatus", permission.CheckPermission("system:ossConfig:edit"),
		operlog.Log(ossConfigStatusLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.ChangeStatus)
	ossConfig.DELETE("/:ossConfigIds", permission.CheckPermission("system:ossConfig:remove"),
		operlog.Log(ossConfigLogTitle, operlog.BusinessTypeDelete), handler.OssConfigApiApp.Remove)

	oss := protected.Group("/oss")
	oss.GET("/list", permission.CheckPermission("system:oss:list"), handler.OssApiApp.List)
	// listByIds/download 与同层的 /:ossIds 段数不同或前缀更具体，gin 能区分，无需调整注册顺序。
	oss.GET("/listByIds/:ossIds", permission.CheckPermission("system:oss:query"),
		handler.OssApiApp.ListByIDs)
	oss.GET("/download/:ossId", permission.CheckPermission("system:oss:download"),
		handler.OssApiApp.Download)
	// upload 不挂防重：同一文件重传是合法诉求，且每次都生成新 key，不存在覆盖。
	// WithoutRequestData 不可省：operlog 采集入参会对 multipart 请求调 ParseForm，
	// 那会把整个文件读进内存再塞进 sys_oper_log.oper_param。
	oss.POST("/upload", permission.CheckPermission("system:oss:upload"),
		operlog.Log(ossLogTitle, operlog.BusinessTypeInsert, operlog.WithoutRequestData()),
		handler.OssApiApp.Upload)
	oss.DELETE("/:ossIds", permission.CheckPermission("system:oss:remove"),
		operlog.Log(ossLogTitle, operlog.BusinessTypeDelete), handler.OssApiApp.Remove)
}

// RegisterPushRoutes 注册推送连接端点。
//
// 推送路径取自配置（push.path，相对部分，默认 /message），注册时拼上 prefix：
// 与业务路由同前缀策略——modular 传 "" 得 /message（nginx 剥 /resource 后匹配），
// standalone 传 "/resource" 得 /resource/message（直连）。
// 不属于 protected 组的中间件链，单独注册在根引擎上。
func RegisterPushRoutes(r *gin.Engine, prefix string) {
	// 未启用推送时不注册，避免前端连上一个只会报错的端点。
	cfg := resourceconfig.Get().Push
	if !cfg.Enabled {
		return
	}

	plugin := sagin.NewPlugin(satoken.Manager())
	path := prefix + cfg.Path
	tokenName := satoken.TokenName()
	// NormalizeQueryToken 须排在 TokenInterceptor 之前：EventSource
	// 不能自定义请求头，token 只能走 query，形如 ?Authorization=Bearer xxx，
	// 而 sa-token-go 的 query 分支不剥 Bearer 前缀，不规范化会一律 401。
	r.GET(path, push.NormalizeQueryToken(tokenName), plugin.TokenInterceptor(),
		checklogin.CheckLogin(), push.Handler())
	// close：前端主动断开时清掉服务端会话，不必等心跳超时才回收。
	r.GET(path+"/close", push.NormalizeQueryToken(tokenName), plugin.TokenInterceptor(),
		push.CloseHandler())
}

// InitRouter 构建并返回 resource 进程的 gin 引擎(独立部署用)。
// prefix 传空：modular 部署经 nginx 剥 /resource，进程内只注册裸路径。
func InitRouter() *gin.Engine {
	r := gin.New()
	cfg := resourceconfig.Get()

	r.Use(recover.Recover())
	r.Use(cors.CORSWithConfig(cfg.CORS))
	r.Use(trace.TraceIDWithConfig(cfg.TraceID))
	r.Use(repeatablebody.RepeatableBodyWithConfig(cfg.RepeatableBody))
	r.Use(accesslog.AccessLogWithConfig(cfg.AccessLog))
	r.Use(xss.XSSWithConfig(cfg.XSS))

	RegisterRoutes(r, "")
	RegisterPushRoutes(r, "")
	notfound.Register(r)

	return r
}
