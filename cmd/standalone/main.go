// Command standalone 单体部署进程入口
package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth"
	"ruoyi-go-vue-plus/internal/system"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
)

func main() {
	config.Load("configs/application.yaml", "configs/standalone.yaml")

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()
	encrypt.Init()

	// 单体引擎：全局中间件装配一次，auth/system 各自只注册本模块路由。
	// standalone 部署保留 /auth、/system 前缀(与前端直连的网关路径一致)。
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())

	auth.RegisterRoutes(r, "/auth")
	system.RegisterRoutes(r, "/system")

	cfg := config.Get()
	log.Printf("[%s] 监听 %s (auth + system standalone)", cfg.Server.Name, cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatal(err)
	}
}
