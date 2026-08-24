package config

import "time"

// JWT 登录态签发配置。
//
// 只有 Secret 进入运行期路径。另两项的现状见各自的注释 ——
// **它们当前不生效**，留着是因为删掉会改动 yaml 的形状，而那属于另一件事。
type JWT struct {
	// Secret JWT 签名密钥，对应原项目 sa-token.jwt-secret-key（application.yml:97）。
	//
	// 签发（internal/auth/service）与校验（pkg/middleware/auth.go）都用它，
	// 所以**两个进程必须配同一个值** —— 不同则 auth 签的 token 在 system 那边验不过，
	// 症状是「登录成功但每个接口都 401」。
	Secret string `mapstructure:"secret"`

	// ExpireMinutes token 有效期（分钟）。
	//
	// **当前不生效**：token 的绝对有效期取 sys_client.timeout（种子数据 7 天），
	// 在签发时落进 JWT 的 exp —— 对齐原项目（那边同样没有全局 sa-token.timeout，
	// 而是每个客户端各自配）。
	//
	// 保留字段是为了 yaml 兼容与将来可能的「无客户端场景」（如内部服务间调用
	// 自签 token）。改它不会影响任何现有登录，别指望调这个值能延长会话。
	ExpireMinutes int `mapstructure:"expireMinutes"`

	// Header 读 token 的请求头名。
	//
	// **当前不生效**，已由 middleware.auth.header 承担 —— 所有中间件配置都收在
	// middleware 段下、一个中间件一个文件（见 pkg/middleware/README.md
	// 「配置怎么读到的」），头名属于「怎么从请求里取 token」即鉴权中间件的事。
	//
	// 两处都配了 Authorization，取值一致，故不会表现出差异；
	// 但要改头名请改 middleware.auth.header，改这里没有任何效果。
	Header string `mapstructure:"header"`
}

// Expire 返回 token 有效期。
//
// 当前无调用方，理由见 ExpireMinutes 的注释。
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
