// Command system 系统管理模块进程入口，监听端口见 configs/system.yaml。
package main

import (
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/database"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/encrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/oplog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/push"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/system"
	systemconfig "ruoyi-go-vue-plus/internal/system/config"
	operlogservice "ruoyi-go-vue-plus/internal/system/service/operlog"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

func main() {
	systemconfig.Load("configs/system.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.OverrideGinCodec()

	cfg := systemconfig.Get()
	database.Init(cfg.Datasource)
	defer database.CloseDefault()

	redis.Init(cfg.Redis)
	defer redis.CloseDefault()

	satoken.Init(cfg.SAToken)
	encrypt.Init(cfg.APIEncrypt)
	// 主键发号器，各业务表主键无 auto_increment，插入前必须就绪。
	snowflake.Init(cfg.Snowflake)
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init(cfg.SAToken.TokenName)
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(operlogservice.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init(cfg.Push)
	defer push.Shutdown()

	r := system.InitRouter()
	system.InitServer(r)
}
