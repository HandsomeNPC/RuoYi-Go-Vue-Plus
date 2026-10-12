package main

import (
	"github.com/HandsomeNPC/ruoyi-common-go/captchahelper"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/encrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/oplog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/ratelimiter"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/social"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/auth"
	authconfig "ruoyi-go-vue-plus/internal/auth/config"
	operlogservice "ruoyi-go-vue-plus/internal/system/service/operlog"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

func main() {
	authconfig.Load("configs/auth.yaml")

	// 须在首个 c.JSON / 参数绑定之前接管 gin 的 JSON codec(雪花 id 按值转字符串)。
	jsonx.OverrideGinCodec()

	cfg := authconfig.Get()
	database.Init(cfg.Datasource)
	defer database.Close()

	redis.Init(cfg.Redis)
	defer redis.Close()

	satoken.Init(cfg.SAToken)
	encrypt.Init(cfg.APIEncrypt)
	// 主键发号器，落库前必须就绪(登录日志插入用)。
	snowflake.Init(cfg.Snowflake)
	// 只构造绘图驱动、不碰 Redis，无前置依赖；校验时才读 Redis。
	captchahelper.Init(cfg.Captcha)
	// 依赖 redis(限流计数存 Redis)，须在 redis.Init 之后。
	ratelimiter.Init()
	// 依赖 redis(防重键存 Redis)，须在 redis.Init 之后。
	// auth 当前无 RepeatSubmit 路由，此处为与其他进程对称、避免将来漏初始化。
	repeatsubmit.Init(cfg.SAToken.TokenName)
	// 三方登录，依赖 redis(state 存 Redis)，须在 redis.Init 之后。
	social.Init(cfg.Social)
	// 同理：auth 当前无 @Log 路由，但漏注册时 pkg/oplog 只打一行日志就丢弃事件，
	// 不会报错——保持对称比事后排查静默丢日志省事。
	oplog.Init(operlogservice.RecordOper)

	r := auth.InitRouter()
	auth.InitServer(r)
}
