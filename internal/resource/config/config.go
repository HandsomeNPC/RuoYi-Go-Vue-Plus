// Package config resource 进程的配置加载。
//
// 自行 viper 解析本模块自包含 yaml，按需只解析 resource 用到的子配置。
// 含 Push/Mail/SMS（验证码与推送），不含 APIEncrypt/User/Captcha/Social。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/database"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/mail"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/push"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/smshelper"

	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
)

// ServerConfig resource 进程的服务配置。
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

// Config resource 进程用到的配置子集。含 Push/Mail/SMS，不含 APIEncrypt/User/Captcha/Social。
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
	I18n           middleware.I18nConfig           `mapstructure:"i18n"`
	Push           push.PushConfig                 `mapstructure:"push"`
	Mail           mail.MailConfig                 `mapstructure:"mail"`
	SMS            smshelper.SMSConfig             `mapstructure:"sms"`
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
	middleware.DefaultI18n().SetDefaults(v)
	push.DefaultPush().SetDefaults(v)
	mail.DefaultMail().SetDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("resource/config: 读取 %s 失败: %w", path, err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("resource/config: 解析配置失败: %w", err))
	}

	for _, fn := range []func() error{
		cfg.Server.Validate,
		cfg.Datasource.Validate,
		cfg.Redis.Validate,
		cfg.SAToken.Validate,
		cfg.CORS.Validate,
		cfg.XSS.Validate,
		cfg.AccessLog.Validate,
		cfg.TraceID.Validate,
		cfg.RepeatableBody.Validate,
		cfg.I18n.Validate,
		cfg.Push.Validate,
		cfg.Mail.Validate,
	} {
		if err := fn(); err != nil {
			panic(fmt.Errorf("resource/config: %w", err))
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
		panic("resource/config: 尚未初始化，请先调用 Load")
	}
	return c
}
