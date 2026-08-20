// Package config 应用配置加载。
//
// 读取 yaml 配置文件并绑定到结构体。各子配置分文件定义：
// server.go / datasource.go / redis.go / jwt.go，各自实现 validate()。
//
// Load 接收多个文件路径，按传入顺序依次合并，后者覆盖前者：
//
//	config.Load("configs/application.yaml", "configs/system.yaml")
//
// 公共配置放 application.yaml，进程独有配置(端口等)放 <module>.yaml。
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 应用完整配置。
type Config struct {
	Server     Server     `mapstructure:"server"`
	Datasource Datasource `mapstructure:"datasource"`
	Redis      Redis      `mapstructure:"redis"`
	JWT        JWT        `mapstructure:"jwt"`
}

// Load 按顺序读取并合并多个 yaml 配置文件，后者覆盖前者。
//
// 至少需要一个路径。任一文件不存在或格式错误都会返回错误，
// 合并完成后逐个子配置执行 validate。
func Load(paths ...string) (*Config, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("config: 至少需要一个配置文件路径")
	}

	v := viper.New()
	v.SetConfigType("yaml")

	for i, path := range paths {
		v.SetConfigFile(path)

		// 第一个文件用 ReadInConfig 建立基准，后续用 MergeInConfig 叠加覆盖。
		read := v.MergeInConfig
		if i == 0 {
			read = v.ReadInConfig
		}
		if err := read(); err != nil {
			return nil, fmt.Errorf("config: 读取 %s 失败: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: 解析配置失败: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate 依次校验各子配置。
func (c *Config) validate() error {
	validators := []func() error{
		c.Server.validate,
		c.Datasource.validate,
		c.Redis.validate,
		c.JWT.validate,
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
