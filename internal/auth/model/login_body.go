package model

// LoginBody 通用登录请求对象，封装客户端、授权类型和验证码信息
// （对应 Java org.dromara.common.core.domain.model.LoginBody）。
// 只含公共字段；各登录方式各自扩展（如 PasswordLoginBody）。
type LoginBody struct {
	ClientID  string `json:"clientId" binding:"required"`
	GrantType string `json:"grantType" binding:"required"`

	Code string `json:"code"`
	UUID string `json:"uuid"`
}
