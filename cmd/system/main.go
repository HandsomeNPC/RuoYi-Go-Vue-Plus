// Command system 系统管理模块进程入口，默认监听 :8081。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/redis"
)

func main() {
	// 走 run 而非在 main 里 log.Fatal，否则 defer 的资源清理会被跳过。
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config.Load("configs/application.yaml", "configs/system.yaml")
	cfg := config.Get()

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	r := gin.New()
	middleware.Register(r)

	// TODO: system.RegisterRoutes(r, deps)

	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
