package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/auth/handler"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/satoken"
)

// RegisterRoutes 将 auth 模块路由注册到给定 gin 引擎(供单体部署复用)。
// 全局中间件(Recover/CORS/TraceID 等)由调用方在引擎级装配，此处只注册模块路由。
//
// prefix 为路由前缀：
//   - 单体部署传 "/auth"：路由形如 /auth/login、/auth/logout、/auth/ping。
//   - 独立部署传 ""：路由形如 /login、/logout、/ping，由 nginx 代理时去 /auth 前缀。
func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	// 接口组：组级只解析 token 存值，鉴权/加解密由逐路由注解决定。
	g := r.Group(prefix)
	g.Use(plugin.TokenInterceptor())
	{
		g.POST("/login", sagin.Ignore(), encrypt.ApiEncrypt(), handler.AuthApiApp.Login)
		g.POST("/logout", sagin.Ignore(), handler.AuthApiApp.Logout)
		// 验证码为 GET 且需匿名访问，不挂 ApiEncrypt（该注解只作用于 POST/PUT）。
		// 对照 Java @RateLimiter(time=60, count=10, limitType=IP)：同一 IP 每分钟最多取 10 次。
		g.GET("/code", sagin.Ignore(),
			ratelimiter.RateLimiter(time.Minute, 10, ratelimiter.LimitTypeIP, 0, ""),
			handler.AuthApiApp.Code)
		g.GET("/ping", sagin.Ignore(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"module": "auth", "message": "pong"})
		})
	}
}

// InitRouter 构建并返回 auth 进程的 gin 引擎(独立部署用)。
// 独立部署不带 /auth 前缀，交给 nginx 代理时剥离。
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
