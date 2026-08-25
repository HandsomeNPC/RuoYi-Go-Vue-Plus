package model

// PasswordLoginBody 密码登录对象（对应 Java org.dromara.system.api.model.PasswordLoginBody）。
// 内嵌 LoginBody，继承 clientId/grantType/code/uuid 字段。
type PasswordLoginBody struct {
	LoginBody
	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=5,max=30"`
}
