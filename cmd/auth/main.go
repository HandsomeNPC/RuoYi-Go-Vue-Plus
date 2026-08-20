// Command auth 认证模块进程入口，默认监听 :8080。
//
// in-process 复用 system 的 service，故也连接同一数据库。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

func main() {
	cfg, err := config.Load("configs/application.yaml", "configs/auth.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// TODO: database.New(cfg.Datasource) -> redis.New(cfg.Redis)
	// TODO: middleware 注入 -> auth.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
