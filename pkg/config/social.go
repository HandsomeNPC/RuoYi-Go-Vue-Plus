package config

import "github.com/spf13/viper"

// SocialLoginConfig 单个三方平台的配置，对应 yaml social.type.<source>。
//
// 字段只保留已接入平台用得上的五项。Java SocialLoginConfigProperties 还有
// unionId/tenantId/agentId/alipayPublicKey/stackOverflowKey 等，
// 全是未接入平台专属的参数，接入时再补。
type SocialLoginConfig struct {
	// ClientID 应用 ID。
	ClientID string `mapstructure:"clientId"`
	// ClientSecret 应用密钥。
	ClientSecret string `mapstructure:"clientSecret"`
	// RedirectURI 回调地址，指向前端回调页，须与平台控制台登记的完全一致。
	RedirectURI string `mapstructure:"redirectUri"`
	// ServerURL 自托管授权服务器地址，仅私有化部署平台(maxkey/topiam)需要。
	ServerURL string `mapstructure:"serverUrl"`
	// Scopes 请求的权限范围，留空则用各平台的默认值。
	Scopes []string `mapstructure:"scopes"`
}

// Configured 判断该平台是否配了基本凭据。
// 两项缺一即视为未启用——对齐 Java AuthChecker.isSupportedAuth 的口径：
// 它把「配置不全」也归进「不支持该平台」。
func (c SocialLoginConfig) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// SocialConfig 三方登录配置，对应 yaml 的 social 段(Java 侧叫 justauth)。
//
// 整段留空是合法的：三方登录本就是可选功能，未配置的平台由
// /auth/binding/{source} 返回「xx平台账号暂不支持」。
type SocialConfig struct {
	// Type 按平台标识(gitee/github/maxkey/topiam)索引的配置表。
	Type map[string]SocialLoginConfig `mapstructure:"type"`
}

// DefaultSocial 返回默认配置。
//
// 返回空 map 而非预置各平台：viper 的 SetDefault 无法给「键名未知」的 map 项
// 铺默认值，各平台的默认 scope 只能落在 pkg/social 的 provider 里。
func DefaultSocial() SocialConfig {
	return SocialConfig{Type: map[string]SocialLoginConfig{}}
}

// setDefaults 把默认值铺给 viper。
func (c SocialConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("social.type", c.Type)
}

// validate 校验三方登录配置。
//
// 只校验「填了一半」的平台：整项留空表示不启用，跳过即可；
// 但配了 clientId 却漏了 secret/回调地址，是会在运行期才暴露的配置事故。
func (c SocialConfig) validate() error {
	for source, lc := range c.Type {
		if lc.ClientID == "" && lc.ClientSecret == "" {
			continue
		}
		if lc.ClientID == "" {
			return errMissing("social.type." + source + ".clientId")
		}
		if lc.ClientSecret == "" {
			return errMissing("social.type." + source + ".clientSecret")
		}
		if lc.RedirectURI == "" {
			return errMissing("social.type." + source + ".redirectUri")
		}
	}
	return nil
}
