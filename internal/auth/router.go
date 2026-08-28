package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/auth/handler"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/satoken"
)

// InitRouter 构建并返回 auth 进程的 gin 引擎。
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

	// /auth 接口组：组级只解析 token 存值，鉴权/加解密由逐路由注解决定。
	g := r.Group("/auth")
	g.Use(plugin.TokenInterceptor())
	{
		g.POST("/login", sagin.Ignore(), encrypt.ApiEncrypt(), handler.AuthApiApp.Login)
		g.POST("/logout", sagin.Ignore(), handler.AuthApiApp.Logout)
		g.GET("/ping", sagin.Ignore(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
		})
	}

	return r
}
