// Command system 系统管理模块进程入口，默认监听 :8081。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/redis"
)

func main() {
	// 走 run 而非在 main 里 log.Fatal，否则 defer 的资源清理会被跳过。
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load("configs/application.yaml", "configs/system.yaml")
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

	if err := redis.Init(cfg.Redis); err != nil {
		return err
	}
	defer func() {
		if err := redis.CloseDefault(); err != nil {
			log.Printf("关闭 Redis 连接失败: %v", err)
		}
	}()
	log.Printf("[%s] Redis 已连接 %s db=%d",
		cfg.Server.Name, cfg.Redis.Addr(), cfg.Redis.DB)

	// TODO: middleware 注入 -> system.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
