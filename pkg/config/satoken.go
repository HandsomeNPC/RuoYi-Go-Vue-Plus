package config

import "github.com/spf13/viper"

// SAToken sa-token-go 鉴权配置，对应 yaml 的 satoken 段。
//
// 仅暴露 4 个可配项(token 名、并发登录、共享 token、jwt 密钥)；其余策略
// (JWT 模式、30 天超时、仅读 Header、auto-renew、键前缀 satoken 等)在
// pkg/satoken.Init 里固定写死。多进程(auth / system)必须读同一份值，否则
// auth 签的 token 在 system 那边验不过。
type SATokenConfig struct {
	// TokenName 读取 token 的请求头名，对应 sa-token.token-name。
	TokenName string `mapstructure:"tokenName"`
	// IsConcurrent 是否允许同一账号并发登录(为true时允许一起登录，为false时新登录挤掉旧登录)。
	IsConcurrent bool `mapstructure:"isConcurrent"`
	// IsShare 多端登录时是否共用一个token(为true时所有登录共用一个token，为false时每次登录新建一个token)。
	IsShare bool `mapstructure:"isShare"`
	// JwtSecretKey JWT 签名密钥，必填(固定 JWT 模式)。
	JwtSecretKey string `mapstructure:"jwtSecretKey"`
}

// DefaultSAToken 返回默认配置。
func DefaultSAToken() SATokenConfig {
	return SATokenConfig{
		TokenName:    "Authorization",
		IsConcurrent: true,
		IsShare:      false,
		JwtSecretKey: "",
	}
}

// setDefaults 把默认值铺给 viper。
func (s SATokenConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("satoken.tokenName", s.TokenName)
	v.SetDefault("satoken.isConcurrent", s.IsConcurrent)
	v.SetDefault("satoken.isShare", s.IsShare)
	v.SetDefault("satoken.jwtSecretKey", s.JwtSecretKey)
}

// validate 校验 sa-token 配置。
func (s SATokenConfig) validate() error {
	if s.TokenName == "" {
		return errMissing("satoken.tokenName")
	}
	if s.JwtSecretKey == "" {
		return errMissing("satoken.jwtSecretKey")
	}
	return nil
}
