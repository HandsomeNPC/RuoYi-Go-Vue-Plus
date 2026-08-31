package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/system/handler"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/satoken"
)

// RegisterRoutes 将 system 模块路由注册到给定 gin 引擎(供单体部署复用)。
// 全局中间件(Recover/CORS/TraceID 等)由调用方在引擎级装配，此处只注册模块路由。
//
// prefix 为路由前缀：
//   - 单体部署传 "/system"：探针 /system/ping，受保护路由形如 /system/xxx。
//   - 独立部署传 ""：探针 /ping，受保护路由形如 /xxx，由 nginx 代理时去 /system 前缀。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	// 公开路由(免鉴权)：探针
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "system", "message": "pong"})
	})

	// 受保护路由：组级只解析 token 存值(供 handler 用 sagin.GetTokenFromCtx 取)，
	// 鉴权由逐路由注解决定。落地业务路由时按需加：
	//   sagin.CheckLogin()                        // 只要登录
	//   sagin.CheckPermission("system:user:list") // 登录 + 权限码
	protected := r.Group(prefix)
	protected.Use(plugin.TokenInterceptor())
	// /system/user：对应 Java SysUserController 的 @RequestMapping("/system/user")。
	user := protected.Group("/user")
	// getInfo 仅需登录（对照 Java getInfo 无 @SaCheckPermission）。
	user.GET("/getInfo", sagin.CheckLogin(), handler.UserApiApp.GetInfo)
}

// InitRouter 构建并返回 system 进程的 gin 引擎(独立部署用)。
// 独立部署不带 /system 前缀，交给 nginx 代理时剥离。
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
