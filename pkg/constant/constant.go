package constant

// 通用基础常量，移植原项目 Constants。
const (
	ConstantUTF8  = "UTF-8"
	ConstantGBK   = "GBK"
	ConstantWWW   = "www." // www 主域，原项目为小写
	ConstantHTTP  = "http://"
	ConstantHTTPS = "https://"

	ConstantSuccess = "0" // 通用成功标识
	ConstantFail    = "1" // 通用失败标识

	ConstantLoginSuccess = "Success"
	ConstantLogout       = "Logout"
	ConstantRegister     = "Register"
	ConstantLoginFail    = "Error"

	ConstantCaptchaExpiration       = 2 // 验证码有效期（分钟）
	ConstantTopParentID       int64 = 0 // 顶级父级 id
	ConstantEncryptHeader           = "ENC_"
)
