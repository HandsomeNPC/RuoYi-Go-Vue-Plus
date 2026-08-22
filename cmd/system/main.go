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

	// 不用 gin.Default()：它自带的 gin.Recovery() 只写 500 空响应，
	// 与 middleware.Recover() 的职责重叠且语义不符（详见 pkg/middleware/README.md）。
	// Recover 放最外层，才能兜住后续中间件自身的 panic。
	r := gin.New()
	r.Use(middleware.Recover())
	// CORS 必须在鉴权之前：预检是 OPTIONS 且不带 token，
	// 先过鉴权会被 401，浏览器就拿不到跨域头了。
	r.Use(middleware.CORS())
	// TraceID 紧跟 CORS：越靠前，越多日志能带上链路 id。
	// 放在 CORS 之后是因为跨域预检会被 CORS 就地终止，不进业务也不需要 id。
	r.Use(middleware.TraceID())
	// RepeatableBody 必须在 AccessLog 之前：body 是一次性的 io.ReadCloser，
	// 日志中间件读完 handler 就绑不到参数了。
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	// XSS 必须在 AccessLog 之后、且在任何读 gin 参数的一环之前：
	// 日志要记原始报文（排查取证看的是攻击者到底发了什么），而 gin 的
	// c.Query 一旦缓存过 URL.Query()，XSS 再改 RawQuery 就静默失效了。
	r.Use(middleware.XSS())
	// I18n 必须在鉴权之前：鉴权失败的提示文案要走词条，得先有语言。
	r.Use(middleware.I18n())

	// TODO: system.RegisterRoutes(r, deps)

	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": cfg.Server.Name, "message": "pong"})
	})

	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	return r.Run(cfg.Server.Addr)
}
