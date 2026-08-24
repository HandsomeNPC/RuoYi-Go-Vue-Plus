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
	// 加载失败直接 panic 在 config.Load 里，不回传 error。
	config.Load("configs/application.yaml", "configs/system.yaml")
	cfg := config.Get()

	database.Init()
	// defer 留在 run() 而非 Init 内：Init 一返回 defer 就触发，
	// 会立刻把连接关掉。跟着 r.Run 的退出收尾才是对的。
	// 关闭失败由 CloseDefault 自己打日志，这里不必再兜一层 if。
	defer database.CloseDefault()

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
	// 它读 config.Get()，所以必须在 config.Load 之后调用。
	middleware.Register(r)

	// TODO: system.RegisterRoutes(r, deps)

	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
