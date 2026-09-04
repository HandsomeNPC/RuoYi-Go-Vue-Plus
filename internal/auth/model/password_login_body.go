package model

// PasswordLoginBody 密码登录对象，内嵌 LoginBody。
type PasswordLoginBody struct {
	LoginBody
	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=5,max=30"`
}
