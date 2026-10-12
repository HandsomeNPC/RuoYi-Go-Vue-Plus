// Package config system 进程的配置加载。
//
// 自行 viper 解析本模块自包含 yaml，按需只解析 system 用到的子配置。
// 类型由各 owning pkg 提供（redis.RedisConfig / database.DatasourceConfig 等），
// ServerConfig / UserConfig 为本模块自有。
package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/HandsomeNPC/ruoyi-common-go/database"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/apiencrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken"

	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
)

// ServerConfig system 进程的服务配置。
type ServerConfig struct {
	Name string `mapstructure:"name"` // 模块名
	Addr string `mapstructure:"addr"` // 监听地址，如 :8080
}

// Validate 校验服务配置。
func (s ServerConfig) Validate() error {
	if s.Addr == "" {
		return fmt.Errorf("server.addr 未配置")
	}
	return nil
}

// UserConfig 用户相关业务配置，对应 yaml 的 user 段。
// auth 复用本类型（密码策略两侧须一致）。
type UserConfig struct {
	Password UserPassword `mapstructure:"password"`
}

// UserPassword 密码策略。
type UserPassword struct {
	// MaxRetryCount 密码最大错误次数，达到即锁定。
	MaxRetryCount int `mapstructure:"maxRetryCount"`
	// LockTime 锁定时长（分钟）。
	LockTime int `mapstructure:"lockTime"`
}

// DefaultUser 返回用户默认配置。
func DefaultUser() UserConfig {
	return UserConfig{
		Password: UserPassword{
			MaxRetryCount: 5,
			LockTime:      10,
		},
	}
}

// Lock 返回锁定时长。
func (p UserPassword) Lock() time.Duration {
	return time.Duration(p.LockTime) * time.Minute
}

// SetDefaults 把默认值铺给 viper。
func (u UserConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("user.password.maxRetryCount", u.Password.MaxRetryCount)
	v.SetDefault("user.password.lockTime", u.Password.LockTime)
}

// Validate 校验用户配置。
func (u UserConfig) Validate() error {
	if u.Password.MaxRetryCount <= 0 {
		return fmt.Errorf("user.password.maxRetryCount 必须大于 0")
	}
	if u.Password.LockTime <= 0 {
		return fmt.Errorf("user.password.lockTime 必须大于 0")
	}
	return nil
}

// Config system 进程用到的配置子集。
//
// 不含 Push：推送的发布端(push.Publish)无状态、只用 Redis，
// 会话管理与端点只在 resource 进程起，故 system 侧无需推送配置。
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
	APIEncrypt     apiencrypt.APIEncryptConfig         `mapstructure:"apiEncrypt"`
	User           UserConfig                          `mapstructure:"user"`
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
	DefaultUser().SetDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("system/config: 读取 %s 失败: %w", path, err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("system/config: 解析配置失败: %w", err))
	}

	for _, fn := range []func() error{
		cfg.Server.Validate,
		cfg.Datasource.Validate,
		cfg.SAToken.Validate,
		cfg.User.Validate,
	} {
		if err := fn(); err != nil {
			panic(fmt.Errorf("system/config: %w", err))
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
		panic("system/config: 尚未初始化，请先调用 Load")
	}
	return c
}
