// Command resource 资源模块进程入口，监听端口见 configs/resource.yaml。
//
// 承载 Java 侧 /resource/* 的全部接口：OSS 文件上传/下载/列表/删除、
// 对象存储配置 CRUD、消息盒子、以及 SSE/WebSocket 推送连接。
// 消息盒子的业务逻辑仍在 internal/system/service（公告发消息那条路径在 system 进程内），
// 本进程只提供 HTTP 入口，同进程函数调用，不走 HTTP。
package main

import (
	"context"
	"log"

	"ruoyi-go-vue-plus/internal/resource"
	resourceservice "ruoyi-go-vue-plus/internal/resource/service"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/push"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

func main() {
	config.Load("configs/application.yaml", "configs/resource.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.Init()

	database.Init()
	defer database.CloseDefault()

	redis.Init()
	defer redis.CloseDefault()

	satoken.Init()
	// 主键发号器，sys_oss/sys_oss_config 主键无 auto_increment，插入前必须就绪。
	snowflake.Init()
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init()
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(systemservice.OperLogSvcApp.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init()
	defer push.Shutdown()

	// OSS 配置预热：把库里的配置写进缓存并确定默认配置键。
	// 不预热的话首次上传会因取不到默认配置而失败。依赖 database 与 redis。
	if err := resourceservice.OssConfigSvcApp.InitCache(context.Background()); err != nil {
		// 不 fatal：配置表为空或库暂时不可用时，配置管理接口仍应能用来补配置。
		log.Printf("[resource] OSS 配置预热失败，上传功能暂不可用: %v", err)
	}

	r := resource.InitRouter()
	resource.InitServer(r)
}
