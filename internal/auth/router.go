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

func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	g := r.Group(prefix)
	g.Use(plugin.TokenInterceptor())
	{
		g.POST("/login", sagin.Ignore(), encrypt.ApiEncrypt(), handler.AuthApiApp.Login)
		g.POST("/logout", sagin.Ignore(), handler.AuthApiApp.Logout)
		g.GET("/code", sagin.Ignore(),
			ratelimiter.RateLimiter(time.Minute, 10, ratelimiter.LimitTypeIP, 0, ""),
			handler.AuthApiApp.Code)
		g.GET("/ping", sagin.Ignore(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"module": "auth", "message": "pong"})
		})
	}
}

// InitRouter 构建并返回 auth 进程的 gin 引擎(独立部署用)。
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
