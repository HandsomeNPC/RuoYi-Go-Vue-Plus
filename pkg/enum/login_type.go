package enum

import "fmt"

// LoginType 登录方式，对应原项目 enums.LoginType。
//
// 它只有一个用途：登录失败计数时，按登录方式取对应的提示文案
// （原项目 SysLoginService.checkLogin(LoginType, username, supplier)）。
// **不承担登录方式分派**——原项目分派靠 grantType 拼 Spring bean 名
// （IAuthStrategy.login: beanName = grantType + "AuthStrategy"），与本枚举无关。
//
// 与原项目的两点差异：
//
//  1. **多了 Code 字段**。Java 枚举只有下面两个文案字段，靠 enum 实例本身传递
//     （checkLogin(LoginType.PASSWORD, ...)）。Go 没有 enum 单例语义，需要一个
//     标识用于按 grantType 查表，故补上 Code。
//
//  2. **文案不走 i18n**。原枚举存的是 message key（如
//     "user.password.retry.limit.exceed"），由 MessageUtils.message(key, args...)
//     按当前语言渲染。这里直接存中文模板（取自原项目
//     i18n/messages_zh_CN.properties），占位符由 Java MessageFormat 的 {0}/{1}
//     改写为 Go 的 %d。
//
//     `pkg/i18n` 已经落地（词条键就是上面那些），但**没有顺手改过来**：
//     i18n.Msg 需要一个 context.Context 才能知道用哪种语言，而下面两个方法
//     没有这个参数，加上它就要改所有调用方 —— 那属于阶段 1 接登录流程时的事
//     （届时 service 手里本来就有 ctx）。改法是把这两个字段换成词条键、
//     方法签名加 ctx 后转调 i18n.Msg；调用方用的是方法而非裸字段，改动不会外溢。
//
// 注意字段顺序与 Java 构造器相反：Java 是 (retryLimitExceed, retryLimitCount)，
// exceed 在前。对照原项目新增登录方式时别把两个模板填反。
type LoginType struct {
	Code string // 登录方式标识，对应 grantType

	// RetryCountTmpl 未达上限时的提示模板，一个占位符：已错误次数。
	// 对应 Java retryLimitCount。
	RetryCountTmpl string
	// RetryExceedTmpl 达到上限时的提示模板，两个占位符：最大次数、锁定分钟数。
	// 对应 Java retryLimitExceed。
	RetryExceedTmpl string
}

// 登录方式枚举实例。
//
// Social / Xcx 的模板为空，对齐原项目 —— 这两种登录的身份校验在第三方
// （OAuth 平台 / 微信）侧完成，不存在「凭证输错」，原项目的
// SocialAuthStrategy 与 XcxAuthStrategy 都不调用 checkLogin。
// RetryCountMsg / RetryExceedMsg 对空模板返回空串，调用方须判空后再决定是否提示。
//
// LoginTypeSocial 是本项目新增：Java 的 LoginType 只有 4 个值，而 grantType
// 实际有 5 种（password/sms/email/social/xcx，见 sys_client.grant_type 与各
// @Service("xxx"+BASE_NAME)）。Java 能少一个是因为 social 不需要文案、
// 也从不传进 checkLogin；Go 侧 Code 是查表键，缺一个会让 social 查不到，
// 故补齐以保持 Code 与 grantType 的取值域一致。
var (
	LoginTypePassword = LoginType{
		Code:            "password",
		RetryCountTmpl:  "密码输入错误%d次",
		RetryExceedTmpl: "密码输入错误%d次，账户锁定%d分钟",
	}
	LoginTypeSms = LoginType{
		Code:            "sms",
		RetryCountTmpl:  "短信验证码输入错误%d次",
		RetryExceedTmpl: "短信验证码输入错误%d次，账户锁定%d分钟",
	}
	LoginTypeEmail = LoginType{
		Code:            "email",
		RetryCountTmpl:  "邮箱验证码输入错误%d次",
		RetryExceedTmpl: "邮箱验证码输入错误%d次，账户锁定%d分钟",
	}
	LoginTypeSocial = LoginType{
		Code: "social",
	}
	LoginTypeXcx = LoginType{
		Code: "xcx",
	}
)

// loginTypes 全部登录方式。前 4 项顺序与原枚举声明一致，social 为本项目新增。
var loginTypes = []LoginType{
	LoginTypePassword, LoginTypeSms, LoginTypeEmail, LoginTypeSocial, LoginTypeXcx,
}

// LoginTypes 返回全部登录方式的副本。
func LoginTypes() []LoginType {
	return append([]LoginType(nil), loginTypes...)
}

// ParseLoginType 按 Code 精确查找登录方式，未匹配时 ok 为 false。
//
// **仅用于按标识取重试提示文案，不要拿它校验 grantType 的合法性。**
// 原项目里 grantType 是否被允许，取决于该客户端 sys_client.grant_type
// 字段是否包含它（AuthController: !StringUtils.contains(client.getGrantType(), grantType)
// 即报「认证类型异常」），是**按客户端的 DB 配置**而非全局枚举——
// 种子数据中 pc 客户端只允许 "password,social"，app 允许 "password,sms,social"。
// 用本函数代替那道校验，会把「枚举里有但该客户端未开启」的登录方式放进去。
func ParseLoginType(code string) (LoginType, bool) {
	for _, t := range loginTypes {
		if t.Code == code {
			return t, true
		}
	}
	return LoginType{}, false
}

// RetryCountMsg 渲染「已错误 errCount 次」提示，对应原项目
// MessageUtils.message(loginType.getRetryLimitCount(), errorNumber)。
// 该登录方式不做重试计数时返回空串。
func (t LoginType) RetryCountMsg(errCount int) string {
	if t.RetryCountTmpl == "" {
		return ""
	}
	return fmt.Sprintf(t.RetryCountTmpl, errCount)
}

// RetryExceedMsg 渲染「错误次数超限，账户锁定」提示，对应原项目
// MessageUtils.message(loginType.getRetryLimitExceed(), maxRetryCount, lockTime)。
// 该登录方式不做重试计数时返回空串。
func (t LoginType) RetryExceedMsg(maxRetry int, lockMinutes int) string {
	if t.RetryExceedTmpl == "" {
		return ""
	}
	return fmt.Sprintf(t.RetryExceedTmpl, maxRetry, lockMinutes)
}
