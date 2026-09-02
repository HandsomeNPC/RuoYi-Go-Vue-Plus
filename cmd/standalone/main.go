// Command standalone 单体部署进程入口
package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth"
	"ruoyi-go-vue-plus/internal/monitor"
	"ruoyi-go-vue-plus/internal/system"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/push"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/standalone.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.Init()

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()
	encrypt.Init()
	// 主键发号器，各业务表主键无 auto_increment，插入前必须就绪。
	snowflake.Init()
	// 依赖 redis(验证码存 Redis)，须在 redis.Init 之后。
	captcha.Init()
	// 依赖 redis(限流计数存 Redis)，须在 redis.Init 之后。
	ratelimiter.Init()
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init()
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(systemservice.OperLogSvcApp.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init()
	defer push.Shutdown()

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
	// /resource/* 不带模块前缀(对齐 Java)，故在 /system 之外单独注册。
	system.RegisterResourceRoutes(r)
	// monitor 与 system/auth 同构：prefix 传模块名 /monitor，对外即 /monitor/cache。
	monitor.RegisterRoutes(r, "/monitor")

	cfg := config.Get()
	log.Printf("[%s] 监听 %s (auth + system standalone)", cfg.Server.Name, cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatal(err)
	}
}
