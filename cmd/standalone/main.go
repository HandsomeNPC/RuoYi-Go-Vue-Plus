// Command standalone 单体部署进程入口
package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/auth"
	authconfig "ruoyi-go-vue-plus/internal/auth/config"
	"ruoyi-go-vue-plus/internal/monitor"
	monitorconfig "ruoyi-go-vue-plus/internal/monitor/config"
	"ruoyi-go-vue-plus/internal/resource"
	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
	ossconfigservice "ruoyi-go-vue-plus/internal/resource/service/ossconfig"
	"ruoyi-go-vue-plus/internal/system"
	systemconfig "ruoyi-go-vue-plus/internal/system/config"
	operlogservice "ruoyi-go-vue-plus/internal/system/service/operlog"
	"ruoyi-go-vue-plus/pkg/captcha"
	pkgconfig "ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/mail"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/push"
	"ruoyi-go-vue-plus/pkg/ratelimiter"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/sms"
	"ruoyi-go-vue-plus/pkg/social"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

func main() {
	startAt := time.Now()
	// 单体同进程跑全四模块，各模块 config 都须 Load（各模块 server/router 读各自子集）。
	// 桥接：首个 Load 即填充 pkg/config 旧全局，供中间件无参包装读取；后续 Load 覆盖同值。
	path := "configs/standalone.yaml"
	authconfig.Load(path)
	systemconfig.Load(path)
	monitorconfig.Load(path)
	resourceconfig.Load(path)

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.OverrideGinCodec()

	// 单体读 pkg/config 全局（standalone.yaml 含全部段）拿子配置传各 Init；
	// 上面四个模块 Load 已各自缓存子集供各模块 server/router 读取。
	cfg := pkgconfig.Get()
	database.Init(cfg.Datasource)
	defer database.CloseDefault()

	redis.Init(cfg.Redis)
	defer redis.CloseDefault()

	satoken.Init(cfg.SAToken)
	encrypt.Init(cfg.APIEncrypt)
	// 主键发号器，各业务表主键无 auto_increment，插入前必须就绪。
	snowflake.Init(cfg.Snowflake)
	// 依赖 redis(验证码存 Redis)，须在 redis.Init 之后。
	captcha.Init(cfg.Captcha)
	// 依赖 redis(限流计数存 Redis)，须在 redis.Init 之后。
	ratelimiter.Init()
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	repeatsubmit.Init(cfg.SAToken.TokenName)
	// 验证码的两个发送通道，只依赖 config。未配置时接口返回提示而非报错。
	mail.Init(cfg.Mail)
	sms.Init(cfg.SMS)
	// 三方登录，依赖 redis(state 存 Redis)，须在 redis.Init 之后。
	social.Init(cfg.Social)
	// 操作日志落库实现反向注册给 pkg/oplog(pkg 不依赖 internal/service)，
	// 依赖 database 与 snowflake。
	oplog.Init(operlogservice.RecordOper)
	// 推送会话管理器，依赖 redis(跨实例分发走 Redis 订阅)，须在 redis.Init 之后。
	push.Init(cfg.Push)
	defer push.Shutdown()

	// OSS 配置预热：把库里的配置写进缓存并确定默认配置键。
	// 不预热的话首次上传会因取不到默认配置而失败。依赖 database 与 redis。
	if err := ossconfigservice.InitCache(context.Background()); err != nil {
		// 不 fatal：配置表为空或库暂时不可用时，配置管理接口仍应能用来补配置。
		log.Printf("[standalone] OSS 配置预热失败，上传功能暂不可用: %v", err)
	}

	// 单体引擎：全局中间件装配一次，auth/system 各自只注册本模块路由。
	// standalone 部署保留 /auth、/system 前缀(与前端直连的网关路径一致)。
	mw := pkgconfig.Get()
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.CORSWithConfig(mw.CORS))
	r.Use(middleware.TraceIDWithConfig(mw.TraceID))
	r.Use(middleware.RepeatableBodyWithConfig(mw.RepeatableBody))
	r.Use(middleware.AccessLogWithConfig(mw.AccessLog))
	r.Use(middleware.XSSWithConfig(mw.XSS))
	r.Use(middleware.I18nWithConfig(mw.I18n))

	auth.RegisterRoutes(r, "/auth")
	system.RegisterRoutes(r, "/system")
	// resource：standalone 无网关，进程内自带 /resource 前缀，等价于网关 StripPrefix=1 后的效果。
	// 推送端点靠 prefix 拼 push.path（/message），故这里也得传 /resource → /resource/message。
	resource.RegisterRoutes(r, "/resource")
	resource.RegisterPushRoutes(r, "/resource")
	// monitor 与 system/auth 同构：prefix 传模块名 /monitor，对外即 /monitor/cache。
	monitor.RegisterRoutes(r, "/monitor")

	serverCfg := pkgconfig.Get().Server
	log.Printf("[%s] 监听 %s (auth + system standalone)", serverCfg.Name, serverCfg.Addr)
	cost := time.Since(startAt)
	log.Printf("[standalone] 启动完成，耗时: %s", cost)
	if err := r.Run(serverCfg.Addr); err != nil {
		log.Fatal(err)
	}
}
