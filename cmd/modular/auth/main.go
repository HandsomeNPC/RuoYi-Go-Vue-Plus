package main

import (
	"ruoyi-go-vue-plus/internal/auth"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/auth.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.Init()

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()
	encrypt.Init()
	// 主键发号器，落库前必须就绪(登录日志插入用)。
	snowflake.Init()
	// 依赖 redis(验证码存 Redis)，须在 redis.Init 之后。
	captcha.Init()
	// 依赖 redis(限流计数存 Redis)，须在 redis.Init 之后。
	ratelimiter.Init()
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	// auth 当前无 RepeatSubmit 路由，此处为与其他进程对称、避免将来漏初始化。
	repeatsubmit.Init()
	// 同理：auth 当前无 @Log 路由，但漏注册时 pkg/oplog 只打一行日志就丢弃事件，
	// 不会报错——保持对称比事后排查静默丢日志省事。
	oplog.Init(systemservice.OperLogSvcApp.RecordOper)

	r := auth.InitRouter()
	auth.InitServer(r)
}
