// Package config resource 进程的配置加载。
//
// 自行 viper 解析本模块自包含 yaml，按需只解析 resource 用到的子配置。
// 含 Push/Mail/SMS（验证码与推送），不含 APIEncrypt/User/Captcha/Social。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/mailhelper"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/smshelper"
	"ruoyi-go-vue-plus/internal/resource/push"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
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
	Server         ServerConfig                        `mapstructure:"server"`
	Snowflake      snowflake.SnowflakeConfig           `mapstructure:"snowflake"`
	Datasource     database.DatasourceConfig           `mapstructure:"datasource"`
	Redis          redis.RedisConfig                   `mapstructure:"redis"`
	SAToken        satoken.SATokenConfig               `mapstructure:"satoken"`
	CORS           cors.CORSConfig                     `mapstructure:"cors"`
	XSS            xss.XSSConfig                       `mapstructure:"xss"`
	AccessLog      accesslog.AccessLogConfig           `mapstructure:"accessLog"`
	TraceID        trace.TraceIDConfig                 `mapstructure:"traceId"`
	RepeatableBody repeatablebody.RepeatableBodyConfig `mapstructure:"repeatableBody"`
	Push           push.PushConfig                     `mapstructure:"push"`
	Mail           mailhelper.MailConfig               `mapstructure:"mail"`
	SMS            smshelper.SMSConfig                 `mapstructure:"sms"`
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


	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("resource/config: 读取 %s 失败: %w", path, err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("resource/config: 解析配置失败: %w", err))
	}

	// Push 的默认值与校验由 push.Init 兜底，不在此处铺 viper 默认值。
	for _, fn := range []func() error{
		cfg.Server.Validate,
		cfg.Datasource.Validate,
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
