// Package config auth 进程的配置加载。
//
// 桥接期：Load 先调 pkg/config.Load 填充旧全局（中间件无参包装仍读它），
// 再缓存本模块的子集视图；Get 返回子集。pkg/config 删除后，Load 改为自行 viper 解析。
package config

import (
	"sync"

	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"

	pkgconfig "ruoyi-go-vue-plus/pkg/config"
)

// Config auth 进程用到的配置子集。不含 Push（auth 不发推送），含 Captcha/Social。
// User 复用 pkg/config.UserConfig（与 system 同源，密码策略两侧须一致）。
type Config struct {
	Server         pkgconfig.ServerConfig
	Snowflake      snowflake.SnowflakeConfig
	Datasource     pkgconfig.DatasourceConfig
	Redis          pkgconfig.RedisConfig
	SAToken        pkgconfig.SATokenConfig
	CORS           pkgconfig.CORSConfig
	XSS            pkgconfig.XSSConfig
	AccessLog      pkgconfig.AccessLogConfig
	TraceID        pkgconfig.TraceIDConfig
	RepeatableBody pkgconfig.RepeatableBodyConfig
	I18n           pkgconfig.I18nConfig
	APIEncrypt     pkgconfig.APIEncryptConfig
	User           pkgconfig.UserConfig
	Captcha        pkgconfig.CaptchaConfig
	Social         pkgconfig.SocialConfig
}

var (
	mu      sync.RWMutex
	current *Config
)

// Load 读取自包含 yaml 并缓存本模块配置子集。
func Load(path string) {
	pkgconfig.Load(path)
	c := pkgconfig.Get()
	cfg := &Config{
		Server:         c.Server,
		Snowflake:      c.Snowflake,
		Datasource:     c.Datasource,
		Redis:          c.Redis,
		SAToken:        c.SAToken,
		CORS:           c.CORS,
		XSS:            c.XSS,
		AccessLog:      c.AccessLog,
		TraceID:        c.TraceID,
		RepeatableBody: c.RepeatableBody,
		I18n:           c.I18n,
		APIEncrypt:     c.APIEncrypt,
		User:           c.User,
		Captcha:        c.Captcha,
		Social:         c.Social,
	}
	mu.Lock()
	current = cfg
	mu.Unlock()
}

// Get 返回本模块配置，未 Load 会 panic。
func Get() *Config {
	mu.RLock()
	c := current
	mu.RUnlock()
	if c == nil {
		panic("auth/config: 尚未初始化，请先调用 Load")
	}
	return c
}
