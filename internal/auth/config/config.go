// Package config auth 进程的配置加载。
//
// 自行 viper 解析本模块自包含 yaml，按需只解析 auth 用到的子配置。
// User 复用 internal/system/config.UserConfig（auth 复用 system service，密码策略两侧须一致）。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"

	systemconfig "ruoyi-go-vue-plus/internal/system/config"

	"github.com/HandsomeNPC/ruoyi-common-go/captchahelper"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/encrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/social"

	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
)

// ServerConfig auth 进程的服务配置。
type ServerConfig struct {
	Name string `mapstructure:"name"`
	Addr string `mapstructure:"addr"`
}

// Validate 校验服务配置。
func (s ServerConfig) Validate() error {
	if s.Addr == "" {
		return fmt.Errorf("server.addr 未配置")
	}
	return nil
}

// Config auth 进程用到的配置子集。不含 Push（auth 不发推送），含 Captcha/Social。
type Config struct {
	Server         ServerConfig                    `mapstructure:"server"`
	Snowflake      snowflake.SnowflakeConfig       `mapstructure:"snowflake"`
	Datasource     database.DatasourceConfig       `mapstructure:"datasource"`
	Redis          redis.RedisConfig               `mapstructure:"redis"`
	SAToken        satoken.SATokenConfig           `mapstructure:"satoken"`
	CORS           middleware.CORSConfig           `mapstructure:"cors"`
	XSS            middleware.XSSConfig            `mapstructure:"xss"`
	AccessLog      middleware.AccessLogConfig      `mapstructure:"accessLog"`
	TraceID        middleware.TraceIDConfig        `mapstructure:"traceId"`
	RepeatableBody middleware.RepeatableBodyConfig `mapstructure:"repeatableBody"`
	APIEncrypt     encrypt.APIEncryptConfig        `mapstructure:"apiEncrypt"`
	User           systemconfig.UserConfig         `mapstructure:"user"`
	Captcha        captchahelper.CaptchaConfig     `mapstructure:"captcha"`
	Social         social.SocialConfig             `mapstructure:"social"`
}

var (
	mu      sync.RWMutex
	current *Config
)

// Load 读取自包含 yaml 并缓存本模块配置子集。
func Load(path string) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(path)

	satoken.DefaultSAToken().SetDefaults(v)
	middleware.DefaultCORS().SetDefaults(v)
	middleware.DefaultXSS().SetDefaults(v)
	middleware.DefaultAccessLog().SetDefaults(v)
	middleware.DefaultTraceID().SetDefaults(v)
	middleware.DefaultRepeatableBody().SetDefaults(v)
	encrypt.DefaultAPIEncrypt().SetDefaults(v)
	systemconfig.DefaultUser().SetDefaults(v)
	social.DefaultSocial().SetDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("auth/config: 读取 %s 失败: %w", path, err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("auth/config: 解析配置失败: %w", err))
	}

	for _, fn := range []func() error{
		cfg.Server.Validate,
		cfg.Datasource.Validate,
		cfg.SAToken.Validate,
		cfg.CORS.Validate,
		cfg.XSS.Validate,
		cfg.AccessLog.Validate,
		cfg.TraceID.Validate,
		cfg.RepeatableBody.Validate,
		cfg.APIEncrypt.Validate,
		cfg.User.Validate,
		cfg.Social.Validate,
	} {
		if err := fn(); err != nil {
			panic(fmt.Errorf("auth/config: %w", err))
		}
	}

	mu.Lock()
	current = &cfg
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
