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

	// 不用 gin.Default()：它自带的 gin.Recovery() 只写 500 空响应，
	// 与 middleware.Recover() 的职责重叠且语义不符（详见 pkg/middleware/README.md）。
	// Recover 放最外层，才能兜住后续中间件自身的 panic。
	r := gin.New()
	r.Use(middleware.Recover())
	// TODO: CORS / TraceID / RepeatableBody / AccessLog / XSS / I18n
	// 暂用 gin 自带日志，待 middleware.AccessLog 落地后替换。
	r.Use(gin.Logger())

	// TODO: auth.RegisterRoutes(r, deps)

	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
