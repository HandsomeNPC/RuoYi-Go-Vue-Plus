// Package auth 认证模块(登录/登出/验证码)。
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth/handler"
	"ruoyi-go-vue-plus/internal/auth/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/redis"
)

// InitRouter 构建并返回 auth 进程的 gin 引擎
func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.APIEncrypt())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())
	r.Use(middleware.Auth())

	cfg := config.Get()
	svc := service.NewAuthService(database.DB(), redis.Client(), cfg)
	h := handler.NewAuthHandler(svc, cfg.Middleware.Auth)

	g := r.Group("/auth")
	{
		g.POST("/login", h.Login)
		g.POST("/logout", h.Logout)
	}

	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	return r
}
