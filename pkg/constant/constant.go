package constant

// 通用基础常量。
const (
	ConstantUTF8  = "UTF-8"
	ConstantGBK   = "GBK"
	ConstantWWW   = "www."
	ConstantHTTP  = "http://"
	ConstantHTTPS = "https://"

	ConstantSuccess = "0"
	ConstantFail    = "1"

	ConstantLoginSuccess = "Success"
	ConstantLogout       = "Logout"
	ConstantRegister     = "Register"
	ConstantLoginFail    = "Error"

	ConstantCaptchaExpiration       = 2
	ConstantTopParentID       int64 = 0
	ConstantEncryptHeader           = "ENC_"
)

// ClientIDHeader 客户端标识请求头名，对照 Java LoginHelper.CLIENT_KEY = "clientid"。
// 登录接口的 clientId 走 JSON 体（LoginBody.ClientID），但记登录日志时拿不到已解析的
// body（失败分支在 body 解析前后都可能触发），故与 Java 一样从请求头取。
const ClientIDHeader = "clientid"
