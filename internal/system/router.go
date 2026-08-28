package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/satoken"
)

// InitRouter 构建并返回 system 进程的 gin 引擎。
func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())

	cfg := config.Get()
	plugin := sagin.NewPlugin(satoken.Manager())

	// 公开路由(免鉴权)：探针
	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	// 受保护路由：组级只解析 token 存值(供 handler 用 sagin.GetTokenFromCtx 取)，
	// 鉴权由逐路由注解决定。落地业务路由时按需加：
	//   sagin.CheckLogin()                      // 只要登录
	//   sagin.CheckPermission("system:user:list") // 登录 + 权限码
	protected := r.Group("/system")
	protected.Use(plugin.TokenInterceptor())

	return r
}
