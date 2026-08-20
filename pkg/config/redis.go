package config

import "fmt"

// Redis 缓存/会话配置。
type Redis struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr 返回 Redis 连接地址。
func (r Redis) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// validate 校验 Redis 配置。
func (r Redis) validate() error {
	if r.Host == "" {
		return errMissing("redis.host")
	}
	if r.Port <= 0 {
		return errInvalid("redis.port", "必须大于 0")
	}
	if r.DB < 0 {
		return errInvalid("redis.db", "不能为负数")
	}
	return nil
}
