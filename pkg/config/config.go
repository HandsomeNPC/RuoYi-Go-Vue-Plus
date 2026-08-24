// Package config 应用配置加载。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// Config 应用完整配置。
type Config struct {
	Server     Server     `mapstructure:"server"`
	Datasource Datasource `mapstructure:"datasource"`
	Redis      Redis      `mapstructure:"redis"`
	JWT        JWT        `mapstructure:"jwt"`
	Middleware Middleware `mapstructure:"middleware"`
	User       User       `mapstructure:"user"`
}

// Load 按顺序读取并合并多个 yaml 配置文件，后者覆盖前者，随后写入包级实例。
func Load(paths ...string) {
	if len(paths) == 0 {
		panic(fmt.Errorf("config: 至少需要一个配置文件路径"))
	}

	v := viper.New()
	v.SetConfigType("yaml")

	setMiddlewareDefaults(v)
	setUserDefaults(v)

	for i, path := range paths {
		v.SetConfigFile(path)

		read := v.MergeInConfig
		if i == 0 {
			read = v.ReadInConfig
		}
		if err := read(); err != nil {
			panic(fmt.Errorf("config: 读取 %s 失败: %w", path, err))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("config: 解析配置失败: %w", err))
	}

	if err := cfg.validate(); err != nil {
		panic(err)
	}

	mu.Lock()
	current = &cfg
	mu.Unlock()
}

var (
	mu      sync.RWMutex
	current *Config
)

// Get 返回包级默认实例。未成功 Load 过会 panic。
func Get() *Config {
	mu.RLock()
	cfg := current
	mu.RUnlock()
	if cfg == nil {
		panic("config: 尚未初始化，请先调用 config.Load")
	}
	return cfg
}

// validate 依次校验各子配置。
func (c *Config) validate() error {
	validators := []func() error{
		c.Server.validate,
		c.Datasource.validate,
		c.Redis.validate,
		c.JWT.validate,
		c.Middleware.validate,
		c.User.validate,
	}
	for _, fn := range validators {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// errMissing 构造必填项缺失错误。
func errMissing(key string) error {
	return fmt.Errorf("config: %s 未配置", key)
}

// errInvalid 构造取值非法错误。
func errInvalid(key, reason string) error {
	return fmt.Errorf("config: %s %s", key, reason)
}
