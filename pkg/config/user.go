package config

import (
	"time"

	"github.com/spf13/viper"
)

// User 用户相关业务配置，对应 yaml 的 user 段。
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

// setDefaults 把默认值铺给 viper。
func (u UserConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("user.password.maxRetryCount", u.Password.MaxRetryCount)
	v.SetDefault("user.password.lockTime", u.Password.LockTime)
}

// validate 校验用户配置。
func (u UserConfig) validate() error {
	if u.Password.MaxRetryCount <= 0 {
		return errInvalid("user.password.maxRetryCount", "必须大于 0")
	}
	if u.Password.LockTime <= 0 {
		return errInvalid("user.password.lockTime", "必须大于 0")
	}
	return nil
}
