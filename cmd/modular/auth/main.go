package main

import (
	"ruoyi-go-vue-plus/internal/auth"
	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/auth.yaml")

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

	r := auth.InitRouter()
	auth.InitServer(r)
}
