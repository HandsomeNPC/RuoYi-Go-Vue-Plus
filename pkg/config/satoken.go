package config

import "github.com/spf13/viper"

// SAToken sa-token-go 鉴权配置，对应 yaml 的 satoken 段。
//
// 取代已删除的 jwt 与 middleware.auth 两段：登录态签发与 token 读取策略
// 统一收口于此。多进程(auth / system)必须读同一份值，否则 auth 签的 token
// 在 system 那边验不过。
type SATokenConfig struct {
	// TokenName 读取 token 的请求头/Cookie 名，对应 sa-token.token-name。
	TokenName string `mapstructure:"tokenName"`
	// Timeout token 绝对有效期(秒)。对应 sa-token.timeout。
	Timeout int64 `mapstructure:"timeout"`
	// ActiveTimeout 活跃超时(秒)，<=0 表示不限。对应 sa-token.active-timeout。
	ActiveTimeout int64 `mapstructure:"activeTimeout"`
	// IsConcurrent 是否允许同一账号并发登录。
	IsConcurrent bool `mapstructure:"isConcurrent"`
	// IsShare 多端登录时是否共享同一个 token。
	IsShare bool `mapstructure:"isShare"`
	// TokenStyle token 生成风格：uuid/simple/random-32/random-64/random-128/jwt/hash/timestamp/tik。
	TokenStyle string `mapstructure:"tokenStyle"`
	// JwtSecretKey JWT 签名密钥，仅 TokenStyle=jwt 时必填。
	JwtSecretKey string `mapstructure:"jwtSecretKey"`
	// IsReadHeader 是否从请求头读 token。
	IsReadHeader bool `mapstructure:"isReadHeader"`
	// IsReadCookie 是否从 Cookie 读 token（默认关，避免 CSRF）。
	IsReadCookie bool `mapstructure:"isReadCookie"`
	// IsReadBody 是否从请求体读 token。
	IsReadBody bool `mapstructure:"isReadBody"`
	// AutoRenew 是否在访问时异步续期。
	AutoRenew bool `mapstructure:"autoRenew"`
	// KeyPrefix Redis 键前缀，自动补冒号。
	KeyPrefix string `mapstructure:"keyPrefix"`
	// IsLog 是否输出 sa-token 内部日志。
	IsLog bool `mapstructure:"isLog"`
	// IsPrintBanner 是否启动时打印 banner（多进程会重复打印，默认关）。
	IsPrintBanner bool `mapstructure:"isPrintBanner"`
}

// DefaultSAToken 返回默认配置。
func DefaultSAToken() SATokenConfig {
	return SATokenConfig{
		TokenName:     "Authorization",
		Timeout:       2592000, // 30 天
		ActiveTimeout: 0,       // 不限
		IsConcurrent:  true,
		IsShare:       true,
		TokenStyle:    "uuid",
		IsReadHeader:  true,
		IsReadCookie:  false,
		IsReadBody:    false,
		AutoRenew:     true,
		KeyPrefix:     "satoken",
		IsLog:         false,
		IsPrintBanner: false,
	}
}

// setDefaults 把默认值铺给 viper。
func (s SATokenConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("satoken.tokenName", s.TokenName)
	v.SetDefault("satoken.timeout", s.Timeout)
	v.SetDefault("satoken.activeTimeout", s.ActiveTimeout)
	v.SetDefault("satoken.isConcurrent", s.IsConcurrent)
	v.SetDefault("satoken.isShare", s.IsShare)
	v.SetDefault("satoken.tokenStyle", s.TokenStyle)
	v.SetDefault("satoken.jwtSecretKey", s.JwtSecretKey)
	v.SetDefault("satoken.isReadHeader", s.IsReadHeader)
	v.SetDefault("satoken.isReadCookie", s.IsReadCookie)
	v.SetDefault("satoken.isReadBody", s.IsReadBody)
	v.SetDefault("satoken.autoRenew", s.AutoRenew)
	v.SetDefault("satoken.keyPrefix", s.KeyPrefix)
	v.SetDefault("satoken.isLog", s.IsLog)
	v.SetDefault("satoken.isPrintBanner", s.IsPrintBanner)
}

// validate 校验 sa-token 配置。
func (s SATokenConfig) validate() error {
	if s.TokenName == "" {
		return errMissing("satoken.tokenName")
	}
	if s.Timeout <= 0 {
		return errInvalid("satoken.timeout", "必须大于 0")
	}
	if !validTokenStyle(s.TokenStyle) {
		return errInvalid("satoken.tokenStyle", "取值必须是 uuid/simple/random-32/random-64/random-128/jwt/hash/timestamp/tik")
	}
	if s.TokenStyle == "jwt" && s.JwtSecretKey == "" {
		return errMissing("satoken.jwtSecretKey")
	}
	if !s.IsReadHeader && !s.IsReadCookie && !s.IsReadBody {
		return errInvalid("satoken", "isReadHeader/isReadCookie/isReadBody 至少要开一个")
	}
	return nil
}

// validTokenStyle 判断 token 风格字符串是否合法。
func validTokenStyle(s string) bool {
	switch s {
	case "uuid", "simple", "random-32", "random-64", "random-128",
		"jwt", "hash", "timestamp", "tik":
		return true
	}
	return false
}
