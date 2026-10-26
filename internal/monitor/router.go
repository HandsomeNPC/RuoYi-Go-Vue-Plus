package monitor

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/audit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/notfound"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/operlog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/permission"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/recover"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	monitorconfig "ruoyi-go-vue-plus/internal/monitor/config"
	cachehandler "ruoyi-go-vue-plus/internal/monitor/handler/cache"
	logininfohandler "ruoyi-go-vue-plus/internal/monitor/handler/logininfo"
	onlinehandler "ruoyi-go-vue-plus/internal/monitor/handler/online"
	operloghandler "ruoyi-go-vue-plus/internal/monitor/handler/operlog"
)

const loginInfoLogTitle = "登录日志"

const operLogLogTitle = "操作日志"

const userOnlineLogTitle = "在线用户"

const userDeviceLogTitle = "在线设备"

// RegisterRoutes 注册 monitor 路由。
//
// 与 system/auth 一致：内部路由 /cache、/loginInfo 不带模块名前缀，由调用方传 prefix。
// modular 部署 prefix=""，nginx 在 /monitor/ location 里剥前缀转成 /cache、/loginInfo；
// standalone 部署 prefix="/monitor"，对外即 /monitor/cache、/monitor/loginInfo。
//
// 挂 TokenInterceptor + AuditContext：cache 只读不需 AuditContext，但 loginInfo 的
// 删除/清空/解锁带 @Log，操作日志要记录操作人，AuditContext 把登录态写进 ctx 供 operlog 取。
// 两个中间件放一处统一挂，比按子组分别挂更易维护。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	g := r.Group(prefix)
	g.Use(plugin.TokenInterceptor(), audit.AuditContext())

	cache := g.Group("/cache")
	cache.GET("", permission.CheckPermission("monitor:cache:list"), cachehandler.GetInfo)

	loginInfo := g.Group("/loginInfo")
	loginInfo.GET("/list", permission.CheckPermission("monitor:logininfo:list"), logininfohandler.List)
	loginInfo.POST("/export", permission.CheckPermission("monitor:logininfo:export"),
		operlog.Log(loginInfoLogTitle, operlog.BusinessTypeExport), logininfohandler.Export)
	// clean 与 :infoIds 同层，静态段优先，gin 能区分二者，无需调整注册顺序。
	// 权限码用 remove，两处复用。
	loginInfo.DELETE("/clean", permission.CheckPermission("monitor:logininfo:remove"),
		operlog.Log(loginInfoLogTitle, operlog.BusinessTypeClean), logininfohandler.Clean)
	loginInfo.DELETE("/:infoIds", permission.CheckPermission("monitor:logininfo:remove"),
		operlog.Log(loginInfoLogTitle, operlog.BusinessTypeDelete), logininfohandler.Remove)
	// unlock 需登录即可；
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	loginInfo.GET("/unlock/:userName", permission.CheckPermission("monitor:logininfo:unlock"),
		operlog.Log(loginInfoLogTitle, operlog.BusinessTypeOther),
		repeatsubmit.RepeatSubmit(0, ""), logininfohandler.Unlock)

	operLog := g.Group("/operlog")
	operLog.GET("/list", permission.CheckPermission("monitor:operlog:list"), operloghandler.List)
	operLog.POST("/export", permission.CheckPermission("monitor:operlog:export"),
		operlog.Log(operLogLogTitle, operlog.BusinessTypeExport), operloghandler.Export)
	// clean 与 :operIds 同层，静态段优先，gin 能区分二者，无需调整注册顺序（与 loginInfo 同理）。
	// 权限码用 remove，两处复用。
	operLog.DELETE("/clean", permission.CheckPermission("monitor:operlog:remove"),
		operlog.Log(operLogLogTitle, operlog.BusinessTypeClean), operloghandler.Clean)
	operLog.DELETE("/:operIds", permission.CheckPermission("monitor:operlog:remove"),
		operlog.Log(operLogLogTitle, operlog.BusinessTypeDelete), operloghandler.Remove)

	// online 在线用户监控。getInfo/remove 只需登录(操作自身会话)，list/forceLogout 需权限码。
	// /myself/:tokenId 与 /:tokenId 段数不同，gin 按深度区分，无路由冲突。
	online := g.Group("/online")
	online.GET("/list", permission.CheckPermission("monitor:online:list"), onlinehandler.List)
	online.DELETE("/:tokenId", permission.CheckPermission("monitor:online:forceLogout"),
		operlog.Log(userOnlineLogTitle, operlog.BusinessTypeForce),
		repeatsubmit.RepeatSubmit(0, ""), onlinehandler.ForceLogout)
	online.GET("", onlinehandler.GetInfo)
	online.DELETE("/myself/:tokenId",
		operlog.Log(userDeviceLogTitle, operlog.BusinessTypeForce),
		repeatsubmit.RepeatSubmit(0, ""), onlinehandler.Remove)

	// 探针，与 system/auth 对齐。
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Ok(gin.H{"module": "monitor", "message": "pong"}))
	})
}

// InitRouter 构建并返回 monitor 进程的 gin 引擎(独立部署用)。
func InitRouter() *gin.Engine {
	r := gin.New()
	cfg := monitorconfig.Get()

	r.Use(recover.Recover())
	r.Use(cors.CORSWithConfig(cfg.CORS))
	r.Use(trace.TraceIDWithConfig(cfg.TraceID))
	r.Use(repeatablebody.RepeatableBodyWithConfig(cfg.RepeatableBody))
	r.Use(accesslog.AccessLogWithConfig(cfg.AccessLog))
	r.Use(xss.XSSWithConfig(cfg.XSS))

	RegisterRoutes(r, "")
	notfound.Register(r)

	return r
}
