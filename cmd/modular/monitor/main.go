// Command monitor 缓存监控模块进程入口，监听端口见 configs/monitor.yaml。
//
// monitor 只读 Redis(INFO/DBSize/COMMANDSTATS)，不落库、不发号、不写操作日志，
// 故仅初始化 config/jsonx/redis/satoken 四样。鉴权复用 auth 进程登录时写入
// Redis 的 sa-token 会话，本进程连同一 Redis 即可校验权限码。
package main

import (
	"ruoyi-go-vue-plus/internal/monitor"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken"
)

func main() {
	config.Load("configs/application.yaml", "configs/monitor.yaml")

	// 接管 gin 的 JSON codec：dbSize 等 int64 按值决定是否转字符串。
	jsonx.Init()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()

	r := monitor.InitRouter()
	monitor.InitServer(r)
}
