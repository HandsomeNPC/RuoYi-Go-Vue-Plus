// Command auth 认证模块进程入口，默认监听 :8080。
//
// in-process 复用 system 的 service，故也连接同一数据库。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
)

func main() {
	// 走 run 而非在 main 里 log.Fatal，否则 defer 的资源清理会被跳过。
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load("configs/application.yaml", "configs/auth.yaml")
	if err != nil {
		return err
	}

	if err := database.Init(cfg.Datasource); err != nil {
		return err
	}
	defer func() {
		if err := database.CloseDefault(); err != nil {
			log.Printf("关闭数据库连接失败: %v", err)
		}
	}()
	log.Printf("[%s] 数据库已连接 %s:%d/%s",
		cfg.Server.Name, cfg.Datasource.Host, cfg.Datasource.Port, cfg.Datasource.DBName)

	// TODO: redis.New(cfg.Redis)
	// TODO: middleware 注入 -> auth.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
