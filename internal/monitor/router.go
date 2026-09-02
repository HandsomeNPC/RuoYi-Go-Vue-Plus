package monitor

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/monitor/handler"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/satoken"
)

// RegisterRoutes 注册 monitor 路由。
//
// 与 system/auth 一致：内部路由 /cache 不带模块名前缀，由调用方传 prefix。
// modular 部署 prefix=""，nginx 在 /monitor/ location 里剥前缀转成 /cache；
// standalone 部署 prefix="/monitor"，对外即 /monitor/cache。
//
// 只挂 TokenInterceptor：CheckPermission 靠它把 token 写进 ctx。
// 不挂 AuditContext——monitor 只读不写、无 @Log，省掉每请求一次的会话解包。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	g := r.Group(prefix)
	g.Use(plugin.TokenInterceptor())

	cache := g.Group("/cache")
	cache.GET("", satoken.CheckPermission("monitor:cache:list"), handler.CacheApiApp.GetInfo)

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
