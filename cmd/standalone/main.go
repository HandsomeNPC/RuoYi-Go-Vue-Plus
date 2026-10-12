// Command standalone 单体部署进程入口
package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/captchahelper"
	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/mailhelper"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/ratelimiter"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/oplog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/push"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/smshelper"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/auth"
	authconfig "ruoyi-go-vue-plus/internal/auth/config"
	socialservice "ruoyi-go-vue-plus/internal/auth/service/social"
	"ruoyi-go-vue-plus/internal/monitor"
	monitorconfig "ruoyi-go-vue-plus/internal/monitor/config"
	"ruoyi-go-vue-plus/internal/resource"
	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
	ossconfigservice "ruoyi-go-vue-plus/internal/resource/service/ossconfig"
	"ruoyi-go-vue-plus/internal/system"
	systemconfig "ruoyi-go-vue-plus/internal/system/config"
	operlogservice "ruoyi-go-vue-plus/internal/system/service/operlog"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

func main() {
	startAt := time.Now()
	// 单体同进程跑全四模块，各模块 config 都须 Load（各模块 server/router 读各自子集）。
	path := "configs/standalone.yaml"
	authconfig.Load(path)
	systemconfig.Load(path)
	monitorconfig.Load(path)
	resourceconfig.Load(path)

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.OverrideGinCodec()

	// 共享子配置（datasource/redis/satoken/snowflake）与 auth 专属（apiEncrypt/user/captcha/social）
	// 从 auth 子集取；push/mail/sms 从 resource 子集取——standalone.yaml 各段四模块同源。
	ac := authconfig.Get()
	rc := resourceconfig.Get()
	database.Init(ac.Datasource)
	defer database.Close()

	redis.Init(ac.Redis)
	defer redis.Close()

	satoken.Init(ac.SAToken)
	// 主键发号器，各业务表主键无 auto_increment，插入前必须就绪。
	snowflake.Init(ac.Snowflake)
	// 只构造绘图驱动、不碰 Redis，无前置依赖；校验时才读 Redis。
	captchahelper.Init(ac.Captcha)
	// 依赖 redis(限流计数存 Redis)，须在 redis.Init 之后。
	ratelimiter.Init(redis.Client())
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init(redis.Client(), ac.SAToken.TokenName)
	// 验证码的两个发送通道，只依赖 config。未配置时接口返回提示而非报错。
	mailhelper.Init(rc.Mail)
	// 短信按渠道逐个 init，任一成功 enabled 即 true；全失败 enabled=false、接口返回未开启，不 fatal。
	smshelper.Init(rc.SMS)
	// 三方登录，依赖 redis(state 存 Redis)，须在 redis.Init 之后。
	// 不 fatal：三方登录是可选功能，配置有问题不该拦住整个进程启动。
	if err := socialservice.Init(ac.Social, redis.Client()); err != nil {
		log.Printf("[standalone] 三方登录未就绪: %v", err)
	}
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(operlogservice.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init(rc.Push)
	defer push.Shutdown()

	// OSS 配置预热：把库里的配置写进缓存并确定默认配置键。
	// 不预热的话首次上传会因取不到默认配置而失败。依赖 database 与 redis。
	if err := ossconfigservice.InitCache(context.Background()); err != nil {
		// 不 fatal：配置表为空或库暂时不可用时，配置管理接口仍应能用来补配置。
		log.Printf("[standalone] OSS 配置预热失败，上传功能暂不可用: %v", err)
	}

	// 单体引擎：全局中间件装配一次，auth/system 各自只注册本模块路由。
	// standalone 部署保留 /auth、/system 前缀(与前端直连的网关路径一致)。
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(cors.CORSWithConfig(ac.CORS))
	r.Use(trace.TraceIDWithConfig(ac.TraceID))
	r.Use(repeatablebody.RepeatableBodyWithConfig(ac.RepeatableBody))
	r.Use(accesslog.AccessLogWithConfig(ac.AccessLog))
	r.Use(xss.XSSWithConfig(ac.XSS))

	auth.RegisterRoutes(r, "/auth")
	system.RegisterRoutes(r, "/system")
	// resource：standalone 无网关，进程内自带 /resource 前缀，等价于网关 StripPrefix=1 后的效果。
	// 推送端点靠 prefix 拼 push.path（/message），故这里也得传 /resource → /resource/message。
	resource.RegisterRoutes(r, "/resource")
	resource.RegisterPushRoutes(r, "/resource")
	// monitor 与 system/auth 同构：prefix 传模块名 /monitor，对外即 /monitor/cache。
	monitor.RegisterRoutes(r, "/monitor")

	serverCfg := ac.Server
	log.Printf("[%s] 监听 %s (auth + system standalone)", serverCfg.Name, serverCfg.Addr)
	cost := time.Since(startAt)
	log.Printf("[standalone] 启动完成，耗时: %s", cost)
	if err := r.Run(serverCfg.Addr); err != nil {
		log.Fatal(err)
	}
}
