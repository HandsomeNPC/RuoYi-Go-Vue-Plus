package vo

// LoginVo 登录成功后的令牌信息返回对象。
type LoginVo struct {
	// AccessToken 授权令牌。
	AccessToken string `json:"access_token"`
	// RefreshToken 刷新令牌。
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpireIn 授权令牌 access_token 的有效期（秒）。
	ExpireIn int64 `json:"expire_in"`
	// RefreshExpireIn 刷新令牌 refresh_token 的有效期（秒）。
	RefreshExpireIn int64 `json:"refresh_expire_in,omitempty"`
	// ClientID 应用 id。
	ClientID string `json:"client_id"`
	// Scope 令牌权限。
	Scope string `json:"scope,omitempty"`
	// OpenID 用户 openid。
	OpenID string `json:"openid,omitempty"`
}
