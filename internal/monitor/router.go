package monitor

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/monitor/handler"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// loginInfoLogTitle 登录日志的操作日志模块名，对照 Java @Log(title = "登录日志")。
const loginInfoLogTitle = "登录日志"

// RegisterRoutes 注册 monitor 路由。
//
// 与 system/auth 一致：内部路由 /cache、/loginInfo 不带模块名前缀，由调用方传 prefix。
// modular 部署 prefix=""，nginx 在 /monitor/ location 里剥前缀转成 /cache、/loginInfo；
// standalone 部署 prefix="/monitor"，对外即 /monitor/cache、/monitor/loginInfo。
//
// 挂 TokenInterceptor + AuditContext：cache 只读不需 AuditContext，但 loginInfo 的
// 删除/清空/解锁带 @Log，操作日志要记录操作人，AuditContext 把登录态写进 ctx 供 oplog 取。
// 两个中间件放一处统一挂，比按子组分别挂更易维护。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	g := r.Group(prefix)
	g.Use(plugin.TokenInterceptor(), loginhelper.AuditContext())

	cache := g.Group("/cache")
	cache.GET("", satoken.CheckPermission("monitor:cache:list"), handler.CacheApiApp.GetInfo)

	loginInfo := g.Group("/loginInfo")
	loginInfo.GET("/list", satoken.CheckPermission("monitor:logininfo:list"), handler.LoginInfoApiApp.List)
	loginInfo.POST("/export", satoken.CheckPermission("monitor:logininfo:export"),
		oplog.Log(loginInfoLogTitle, enum.BusinessTypeExport), handler.LoginInfoApiApp.Export)
	// clean 与 :infoIds 同层，静态段优先，gin 能区分二者，无需调整注册顺序。
	// 权限码用 remove：对照 Java @SaCheckPermission("monitor:logininfo:remove") 两处复用。
	loginInfo.DELETE("/clean", satoken.CheckPermission("monitor:logininfo:remove"),
		oplog.Log(loginInfoLogTitle, enum.BusinessTypeClean), handler.LoginInfoApiApp.Clean)
	loginInfo.DELETE("/:infoIds", satoken.CheckPermission("monitor:logininfo:remove"),
		oplog.Log(loginInfoLogTitle, enum.BusinessTypeDelete), handler.LoginInfoApiApp.Remove)
	// unlock 需登录即可（Java @SaCheckPermission("monitor:logininfo:unlock")）；
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	loginInfo.GET("/unlock/:userName", satoken.CheckPermission("monitor:logininfo:unlock"),
		oplog.Log(loginInfoLogTitle, enum.BusinessTypeOther),
		repeatsubmit.RepeatSubmit(0, ""), handler.LoginInfoApiApp.Unlock)

	// 探针，与 system/auth 对齐。
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "monitor", "message": "pong"})
	})
}

// InitRouter 构建并返回 monitor 进程的 gin 引擎(独立部署用)。
func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())

	RegisterRoutes(r, "")

	return r
}
