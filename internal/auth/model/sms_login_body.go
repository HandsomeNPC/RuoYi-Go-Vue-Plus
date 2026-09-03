package model

// SmsLoginBody 短信验证码登录对象（对应 Java SmsLoginBody），内嵌 LoginBody。
type SmsLoginBody struct {
	LoginBody
	// PhoneNumber 手机号。
	PhoneNumber string `json:"phoneNumber" binding:"required"`
	// SmsCode 短信验证码。
	SmsCode string `json:"smsCode" binding:"required"`
}
