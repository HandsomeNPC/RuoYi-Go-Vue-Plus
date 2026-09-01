// Command system 系统管理模块进程入口，监听端口见 configs/system.yaml。
package main

import (
	"ruoyi-go-vue-plus/internal/system"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/system.yaml")

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

	r := system.InitRouter()
	system.InitServer(r)
}
