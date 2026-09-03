package resource

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/resource/handler"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/push"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// ossLogTitle OSS 对象存储的操作日志模块名，对照 Java @Log(title = "OSS对象存储")。
const ossLogTitle = "OSS对象存储"

// ossConfigLogTitle 对象存储配置的操作日志模块名，对照 Java @Log(title = "对象存储配置")。
const ossConfigLogTitle = "对象存储配置"

// ossConfigStatusLogTitle 状态修改单独一个标题，对照 Java @Log(title = "对象存储状态修改")。
const ossConfigStatusLogTitle = "对象存储状态修改"

// RoutePrefix 本模块对外的路径前缀，对齐 Java 的 @RequestMapping("/resource/...")。
//
// 两种部署都用它（不像 system 那样 modular 传空由 nginx 剥）：
// 推送端点走 push.path 这个绝对路径，剥前缀会让它失配。
const RoutePrefix = "/resource"

// RegisterRoutes 注册 resource 路由。
//
// 与 system/auth/monitor 一致：内部路由 /oss、/message 不带模块名前缀，由调用方传 prefix。
// 但两种部署都传 "/resource"，而非 modular 传空——推送端点的路径取自 push.path
// （绝对路径 /resource/message，注册在根引擎上，见 RegisterPushRoutes），
// nginx 若剥掉 /resource 前缀，推送端点就会失配，故本模块的 location 不剥前缀，
// 进程内自带完整路径。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	// 公开路由(免鉴权)：探针，与 system/auth/monitor 对齐。
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "resource", "message": "pong"})
	})

	protected := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	protected.Use(plugin.TokenInterceptor(), loginhelper.AuditContext())

	// 与 Java 一致不校验权限码，仅需登录：消息盒子只返回当前用户自己的消息。
	protected.GET("/message/box", sagin.CheckLogin(), handler.MessageApiApp.GetBox)

	// 短信/邮箱验证码：登录前就要调，必须免鉴权。Java 侧整个 CaptchaController
	// 挂 @SaIgnore，而这两个端点落在带 TokenInterceptor 的组内，故显式 Ignore 放行。
	//
	// 限流对照 Java @RateLimiter(key="#phoneNumber", time=60, count=1)：同一手机号
	// /邮箱 60 秒 1 次。Java 把 emailCode/emailCodeImpl 拆两层使开关关闭时不触发限流，
	// Go 侧限流是 handler 之前的中间件，做不到同样的拆分——后果仅是邮箱功能关闭时
	// 重复请求也占额度，接口本就返回失败，不值得为此把限流下沉进 handler。
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
	// 与 Java 一致，详情页复用 list 权限码而非 query。
	ossConfig.GET("/list", satoken.CheckPermission("system:ossConfig:list"),
		handler.OssConfigApiApp.List)
	ossConfig.GET("/:ossConfigId", satoken.CheckPermission("system:ossConfig:list"),
		handler.OssConfigApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /oss/config/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	ossConfig.POST("", satoken.CheckPermission("system:ossConfig:add"),
		oplog.Log(ossConfigLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.Add)
	ossConfig.PUT("", satoken.CheckPermission("system:ossConfig:edit"),
		oplog.Log(ossConfigLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.Edit)
	// changeStatus 路径更具体，须注册在 PUT "" 之后。与 Java 一致挂防重：
	// 它会把全表刷成非默认再置目标行，重复提交期间存在"一个默认都没有"的窗口。
	ossConfig.PUT("/changeStatus", satoken.CheckPermission("system:ossConfig:edit"),
		oplog.Log(ossConfigStatusLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.OssConfigApiApp.ChangeStatus)
	ossConfig.DELETE("/:ossConfigIds", satoken.CheckPermission("system:ossConfig:remove"),
		oplog.Log(ossConfigLogTitle, enum.BusinessTypeDelete), handler.OssConfigApiApp.Remove)

	oss := protected.Group("/oss")
	oss.GET("/list", satoken.CheckPermission("system:oss:list"), handler.OssApiApp.List)
	// listByIds/download 与同层的 /:ossIds 段数不同或前缀更具体，gin 能区分，无需调整注册顺序。
	oss.GET("/listByIds/:ossIds", satoken.CheckPermission("system:oss:query"),
		handler.OssApiApp.ListByIDs)
	oss.GET("/download/:ossId", satoken.CheckPermission("system:oss:download"),
		handler.OssApiApp.Download)
	// upload 不挂防重(对齐 Java 未标 @RepeatSubmit)：同一文件重传是合法诉求，
	// 且每次都生成新 key，不存在覆盖。
	// WithoutRequestData 不可省：oplog 采集入参会对 multipart 请求调 ParseForm，
	// 那会把整个文件读进内存再塞进 sys_oper_log.oper_param。
	oss.POST("/upload", satoken.CheckPermission("system:oss:upload"),
		oplog.Log(ossLogTitle, enum.BusinessTypeInsert, oplog.WithoutRequestData()),
		handler.OssApiApp.Upload)
	oss.DELETE("/:ossIds", satoken.CheckPermission("system:oss:remove"),
		oplog.Log(ossLogTitle, enum.BusinessTypeDelete), handler.OssApiApp.Remove)
}

// RegisterPushRoutes 注册推送连接端点。
//
// 与业务路由分开：推送路径取自配置（push.path，默认 /resource/message），
// 是个完整绝对路径，不能套 prefix，也不属于 protected 组的中间件链。
func RegisterPushRoutes(r *gin.Engine) {
	// 推送端点按 push.transport 决定走 SSE 还是 WebSocket，路径取配置值。
	// 未启用推送时不注册，避免前端连上一个只会报错的端点。
	cfg := config.Get().Push
	if !cfg.Enabled {
		return
	}

	plugin := sagin.NewPlugin(satoken.Manager())
	// NormalizeQueryToken 须排在 TokenInterceptor 之前：EventSource/WebSocket
	// 不能自定义请求头，token 只能走 query，形如 ?Authorization=Bearer xxx，
	// 而 sa-token-go 的 query 分支不剥 Bearer 前缀，不规范化会一律 401。
	r.GET(cfg.Path, push.NormalizeQueryToken(), plugin.TokenInterceptor(),
		sagin.CheckLogin(), push.Handler())
	// close 对齐 Java SseController.close：前端主动断开时清掉服务端会话，
	// 不必等心跳超时才回收。
	r.GET(cfg.Path+"/close", push.NormalizeQueryToken(), plugin.TokenInterceptor(),
		sagin.CheckLogin(), push.CloseHandler())
}

// InitRouter 构建并返回 resource 进程的 gin 引擎(独立部署用)。
// 传 /resource 前缀，nginx 转发时不剥（推送端点的绝对路径依赖它，见 RegisterRoutes）。
func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())

	RegisterRoutes(r, RoutePrefix)
	RegisterPushRoutes(r)

	return r
}
