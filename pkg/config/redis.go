package config

import (
	"fmt"
	"time"
)

// Redis 缓存/会话/分布式锁配置。
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`

	ClientName      string `mapstructure:"clientName"`      // 连接标识
	PoolSize        int    `mapstructure:"poolSize"`        // 连接池大小，缺省 go-redis 默认(10×CPU)
	MinIdleConns    int    `mapstructure:"minIdleConns"`    // 最小空闲连接
	DialTimeoutMs   int    `mapstructure:"dialTimeoutMs"`   // 建连超时(毫秒)，缺省 5000
	ReadTimeoutMs   int    `mapstructure:"readTimeoutMs"`   // 读超时(毫秒)，缺省 3000
	WriteTimeoutMs  int    `mapstructure:"writeTimeoutMs"`  // 写超时(毫秒)，缺省 3000
	ConnMaxIdleTime int    `mapstructure:"connMaxIdleTime"` // 空闲连接最大存活(秒)
}

// Addr 返回 Redis 连接地址。
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// DialTimeout 返回建连超时，未配置时默认 5s。
func (r RedisConfig) DialTimeout() time.Duration {
	return msOrDefault(r.DialTimeoutMs, 5*time.Second)
}

// ReadTimeout 返回读超时，未配置时默认 3s。
func (r RedisConfig) ReadTimeout() time.Duration {
	return msOrDefault(r.ReadTimeoutMs, 3*time.Second)
}

// WriteTimeout 返回写超时，未配置时默认 3s。
func (r RedisConfig) WriteTimeout() time.Duration {
	return msOrDefault(r.WriteTimeoutMs, 3*time.Second)
}

// MaxIdleTime 返回空闲连接最大存活时长。
func (r RedisConfig) MaxIdleTime() time.Duration {
	return time.Duration(r.ConnMaxIdleTime) * time.Second
}

// msOrDefault 把毫秒配置换算成时长，未配置(<=0)时返回默认值。
func msOrDefault(ms int, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

// validate 校验 Redis 配置。
func (r RedisConfig) validate() error {
	if r.Host == "" {
		return errMissing("redis.host")
	}
	if r.Port <= 0 {
		return errInvalid("redis.port", "必须大于 0")
	}
	if r.DB < 0 {
		return errInvalid("redis.db", "不能为负数")
	}
	if r.PoolSize > 0 && r.MinIdleConns > r.PoolSize {
		return errInvalid("redis.minIdleConns", "不能大于 poolSize")
	}
	return nil
}
