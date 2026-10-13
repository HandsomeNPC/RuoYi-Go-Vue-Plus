// Command resource 资源模块进程入口，监听端口见 configs/resource.yaml。
//
// 承载 /resource/* 的全部接口：OSS 上传/下载/列表/删除、对象存储配置 CRUD、消息盒子、SSE 推送连接。
// 消息盒子的业务逻辑仍在 internal/system/service（公告发消息那条路径在 system 进程内），
// 本进程只提供 HTTP 入口，同进程函数调用，不走 HTTP。
package main

import (
	"context"
	"log"

	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/mailhelper"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/ratelimiter"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/operlog"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/smshelper"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/resource"
	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
	"ruoyi-go-vue-plus/internal/resource/push"
	ossconfigservice "ruoyi-go-vue-plus/internal/resource/service/ossconfig"
	operlogservice "ruoyi-go-vue-plus/internal/system/service/operlog"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

func main() {
	resourceconfig.Load("configs/resource.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.OverrideGinCodec()

	cfg := resourceconfig.Get()
	database.Init(cfg.Datasource)
	defer database.Close()

	redis.Init(cfg.Redis)
	defer redis.Close()

	satoken.Init(cfg.SAToken, redis.Client())
	// 主键发号器，sys_oss/sys_oss_config 主键无 auto_increment，插入前必须就绪。
	snowflake.Init(cfg.Snowflake)
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init(redis.Client(), satoken.TokenName())
	// 依赖 redis(限流计数存 Redis)：短信/邮箱验证码两个端点按手机号/邮箱限流。
	ratelimiter.Init(redis.Client())
	// 验证码的两个发送通道，只依赖 config。未配置时接口返回提示而非报错。
	mailhelper.Init(cfg.Mail)
	// 短信按渠道逐个 init，任一成功 enabled 即 true；全失败 enabled=false、接口返回未开启，不 fatal。
	smshelper.Init(cfg.SMS)
	// 操作日志落库实现反向注册给 pkg/operlog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	operlog.Init(operlogservice.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init(cfg.Push)
	defer push.Shutdown()

	// OSS 配置预热：把库里的配置写进缓存并确定默认配置键。
	// 不预热的话首次上传会因取不到默认配置而失败。依赖 database 与 redis。
	if err := ossconfigservice.InitCache(context.Background()); err != nil {
		// 不 fatal：配置表为空或库暂时不可用时，配置管理接口仍应能用来补配置。
		log.Printf("[resource] OSS 配置预热失败，上传功能暂不可用: %v", err)
	}

	r := resource.InitRouter()
	resource.InitServer(r)
}
