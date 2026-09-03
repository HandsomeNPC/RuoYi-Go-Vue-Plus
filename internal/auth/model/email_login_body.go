package model

// EmailLoginBody 邮箱验证码登录对象（对应 Java EmailLoginBody），内嵌 LoginBody。
type EmailLoginBody struct {
	LoginBody
	// Email 邮箱。
	Email string `json:"email" binding:"required,email"`
	// EmailCode 邮箱验证码。
	EmailCode string `json:"emailCode" binding:"required"`
}
