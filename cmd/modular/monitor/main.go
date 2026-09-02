// Command monitor 监控模块进程入口，监听端口见 configs/monitor.yaml。
//
// 既读 Redis（缓存监控），又落库读 MySQL（登录日志列表/删除/清空）与读写 Redis
// （账户解锁删 pwd_err_cnt 锁定键）。故初始化 database/snowflake/oplog/repeatsubmit：
// database 供登录日志查询/删除/清空，snowflake 因 oplog 落库 sys_oper_log 需主键发号，
// oplog 给 @Log 接口（导出/删除/清空/解锁）记操作日志，repeatsubmit 给 unlock 防重。
// 鉴权复用 auth 进程登录时写入 Redis 的 sa-token 会话，本进程连同一 Redis 即可校验权限码。
package main

import (
	"ruoyi-go-vue-plus/internal/monitor"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/monitor.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.Init()

	redis.Init()
	defer redis.CloseDefault()

	// 登录日志查询/删除/清空走 MySQL。
	database.Init()
	defer database.CloseDefault()

	satoken.Init()

	// oplog 落 sys_oper_log 的主键发号器，须在 oplog.Init 之前就绪。
	snowflake.Init()
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(systemservice.OperLogSvcApp.RecordOper)
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init()

	r := monitor.InitRouter()
	monitor.InitServer(r)
}
