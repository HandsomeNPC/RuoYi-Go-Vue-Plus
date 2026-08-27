package main

import (
	"ruoyi-go-vue-plus/internal/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
)

func main() {
	config.Load("configs/application.yaml", "configs/auth.yaml")

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()

	r := auth.InitRouter()
	auth.InitServer(r)
}
