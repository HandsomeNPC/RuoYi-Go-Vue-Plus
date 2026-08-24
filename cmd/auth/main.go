// Command auth 认证模块进程入口，默认监听 :8080。
//
// in-process 复用 system 的 service，故也连接同一数据库。
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth"
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
	if err := config.Load("configs/application.yaml", "configs/auth.yaml"); err != nil {
		return err
	}
	cfg := config.Get()

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
	r := gin.New()
	// 注册顺序与两个进程的一致性由 middleware.Register 保证。
	// 它读 config.Get() 与 redis.Client()，所以必须在 config.Load 与
	// redis.Init 之后调用 —— 鉴权中间件要用这两者。
	middleware.Register(r)

	// 依赖显式注入：本模块内部不调 database.DB() / redis.Client()。
	// 配置例外 —— RegisterRoutes 自己走 config.Get()（Load 已写入包级实例）。
	// auth 的 service 直接 import system 的 service（同进程函数调用、
	// 无网络开销），故这里传的 DB 与 system 进程连的是同一个库。
	auth.RegisterRoutes(r, auth.Deps{
		DB:    database.DB(),
		Redis: redis.Client(),
	})

	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
