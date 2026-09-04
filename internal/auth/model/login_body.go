package model

// LoginBody 通用登录请求对象。只含公共字段，各登录方式各自扩展。
type LoginBody struct {
	ClientID  string `json:"clientId" binding:"required"`
	GrantType string `json:"grantType" binding:"required"`

	Code string `json:"code"`
	UUID string `json:"uuid"`
}
