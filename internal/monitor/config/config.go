// Package config monitor 进程的配置加载。
//
// 自行 viper 解析本模块自包含 yaml，按需只解析 monitor 用到的子配置（最瘦）。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
)

// ServerConfig monitor 进程的服务配置。
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

// Config monitor 进程用到的配置子集。最瘦：无 APIEncrypt/User/Captcha/Social/Push/Mail/SMS。
type Config struct {
	Server         ServerConfig                        `mapstructure:"server"`
	Snowflake      snowflake.SnowflakeConfig           `mapstructure:"snowflake"`
	Datasource     database.DatasourceConfig           `mapstructure:"datasource"`
	Redis          redis.RedisConfig                   `mapstructure:"redis"`
	SAToken        satoken.SATokenConfig               `mapstructure:"satoken"`
	CORS           middleware.CORSConfig               `mapstructure:"cors"`
	XSS            middleware.XSSConfig                `mapstructure:"xss"`
	AccessLog      middleware.AccessLogConfig          `mapstructure:"accessLog"`
	TraceID        middleware.TraceIDConfig            `mapstructure:"traceId"`
	RepeatableBody repeatablebody.RepeatableBodyConfig `mapstructure:"repeatableBody"`
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
	repeatablebody.DefaultRepeatableBody().SetDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("monitor/config: 读取 %s 失败: %w", path, err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("monitor/config: 解析配置失败: %w", err))
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
	} {
		if err := fn(); err != nil {
			panic(fmt.Errorf("monitor/config: %w", err))
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
		panic("monitor/config: 尚未初始化，请先调用 Load")
	}
	return c
}
