package model

// LoginBody 登录入参。
type LoginBody struct {
	ClientID  string `json:"clientId" binding:"required"`
	GrantType string `json:"grantType" binding:"required"`

	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=5,max=30"`

	Code string `json:"code"`
	UUID string `json:"uuid"`
}

// LoginVo 登录结果。
type LoginVo struct {
	AccessToken string `json:"access_token"`
	ExpireIn    int64  `json:"expire_in"`
	ClientID    string `json:"client_id"`

	RefreshToken    string `json:"refresh_token,omitempty"`
	RefreshExpireIn int64  `json:"refresh_expire_in,omitempty"`
	Scope           string `json:"scope,omitempty"`
	OpenID          string `json:"openid,omitempty"`
}
