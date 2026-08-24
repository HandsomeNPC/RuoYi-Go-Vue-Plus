package config

import (
	"github.com/spf13/viper"
)

// TokenHeader 读取 token 的请求头名。
const TokenHeader = "Authorization"

// TokenPrefix token 值的前缀。
const TokenPrefix = "Bearer"

// ClientIDHeader 客户端标识的请求头/查询串参数名。
const ClientIDHeader = "clientid"

// defaultAuthExcludes 免鉴权路径，Ant 风格。
var defaultAuthExcludes = []string{
	"/*.html",
	"/**/*.html",
	"/**/*.css",
	"/**/*.js",
	"/favicon.ico",
	"/error",
	"/*/api-docs",
	"/*/api-docs/**",
	"/warm-flow-ui/config",
	"/snail-chat/**",
	"/api/snail/chat/**",
	"/auth/**",
}

// Auth 鉴权中间件配置。
type Auth struct {
	// Excludes 免鉴权路径，Ant 风格。为空表示用 defaultAuthExcludes。
	Excludes []string `mapstructure:"excludes"`

	// Header 读取 token 的请求头名，为空表示用 TokenHeader。
	Header string `mapstructure:"header"`

	// TokenPrefix token 前缀，为空表示不使用前缀。
	TokenPrefix string `mapstructure:"tokenPrefix"`

	// ClientIDHeader 客户端标识的头名，为空表示用 ClientIDHeader。
	ClientIDHeader string `mapstructure:"clientIdHeader"`
}

// defaultAuth 返回默认配置。
func defaultAuth() Auth {
	return Auth{
		Excludes:       defaultAuthExcludes,
		Header:         TokenHeader,
		TokenPrefix:    TokenPrefix,
		ClientIDHeader: ClientIDHeader,
	}
}

// setDefaults 把默认值铺给 viper。
func (a Auth) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.auth.excludes", a.Excludes)
	v.SetDefault("middleware.auth.header", a.Header)
	v.SetDefault("middleware.auth.tokenPrefix", a.TokenPrefix)
	v.SetDefault("middleware.auth.clientIdHeader", a.ClientIDHeader)
}

// validate 校验鉴权配置。
func (a Auth) validate() error {
	for _, p := range a.Excludes {
		if p == "" {
			return errInvalid("middleware.auth.excludes", "不能包含空路径")
		}
	}
	return nil
}
