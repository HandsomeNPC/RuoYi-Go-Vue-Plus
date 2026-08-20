// Command system 系统管理模块进程入口，默认监听 :8081。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

func main() {
	cfg, err := config.Load("configs/application.yaml", "configs/system.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// TODO: database.New(cfg.Datasource) -> redis.New(cfg.Redis)
	// TODO: middleware 注入 -> system.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
