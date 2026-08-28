// Command system 系统管理模块进程入口，默认监听 :8081。
package main

import (
	"ruoyi-go-vue-plus/internal/system"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
)

func main() {
	config.Load("configs/application.yaml", "configs/system.yaml")

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()
	encrypt.Init()

	r := system.InitRouter()
	system.InitServer(r)
}
