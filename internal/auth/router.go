// Package auth 认证模块(登录/登出/验证码)。
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth/handler"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/middleware"
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

	g := r.Group("/auth")
	{
		g.POST("/login", handler.AuthApiApp.Login)
		g.POST("/logout", handler.AuthApiApp.Logout)
	}

	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	return r
}
