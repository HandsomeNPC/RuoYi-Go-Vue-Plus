package system

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/middleware"
)

// InitRouter 构建并返回 system 进程的 gin 引擎。
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

	// TODO: 阶段 3 接入 sagin 鉴权中间件（sagin.NewPlugin(...).PathAuthMiddleware / AuthMiddleware）。

	// TODO: 在此挂载 system 路由，阶段 2 落地。

	cfg := config.Get()
	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	return r
}
