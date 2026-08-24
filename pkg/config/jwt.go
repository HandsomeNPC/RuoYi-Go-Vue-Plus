package config

import "time"

// JWT 登录态签发配置。
type JWT struct {
	Secret        string `mapstructure:"secret"`
	ExpireMinutes int    `mapstructure:"expireMinutes"`
	Header        string `mapstructure:"header"`
}

// Expire 返回 token 有效期。
func (j JWT) Expire() time.Duration {
	return time.Duration(j.ExpireMinutes) * time.Minute
}

// validate 校验 JWT 配置。
func (j JWT) validate() error {
	if j.Secret == "" {
		return errMissing("jwt.secret")
	}
	if j.ExpireMinutes <= 0 {
		return errInvalid("jwt.expireMinutes", "必须大于 0")
	}
	return nil
}
