package enum

import (
	"context"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// LoginType 登录方式，对应原项目 enums.LoginType。
//
// 它只有一个用途：登录失败计数时，按登录方式取对应的提示文案
// （原项目 SysLoginService.checkLogin(LoginType, username, supplier)）。
// **不承担登录方式分派**——原项目分派靠 grantType 拼 Spring bean 名
// （IAuthStrategy.login: beanName = grantType + "AuthStrategy"），与本枚举无关。
//
// 与原项目的唯一差异是**多了 Code 字段**。Java 枚举只有下面两个词条键字段，
// 靠 enum 实例本身传递（checkLogin(LoginType.PASSWORD, ...)）。Go 没有 enum
// 单例语义，需要一个标识用于按 grantType 查表，故补上 Code。
//
// 两个文案字段存的是**词条键**而非文案本身，与 Java 一致
// （LoginType.PASSWORD 的 retryLimitExceed 就是 "user.password.retry.limit.exceed"），
// 渲染统一走 pkg/i18n。曾经这里存的是中文模板，那样会让同一批文案在
// 仓库里存两份副本 —— pkg/i18n 那份有与原项目 .properties 的交叉验证兜着，
// 这份没有，两者能对上只能靠巧合。
//
// 注意字段顺序与 Java 构造器相反：Java 是 (retryLimitExceed, retryLimitCount)，
// exceed 在前。对照原项目新增登录方式时别把两个键填反。
type LoginType struct {
	Code string // 登录方式标识，对应 grantType

	// RetryCountKey 未达上限时的提示词条键，该词条有一个占位符：已错误次数。
	// 对应 Java retryLimitCount。
	RetryCountKey string
	// RetryExceedKey 达到上限时的提示词条键，该词条有两个占位符：最大次数、锁定分钟数。
	// 对应 Java retryLimitExceed。
	RetryExceedKey string
}

// 登录方式枚举实例。
//
// Social / Xcx 的词条键为空，对齐原项目（Java 侧 XCX("", "")）—— 这两种登录的
// 身份校验在第三方（OAuth 平台 / 微信）侧完成，不存在「凭证输错」，
// 原项目的 SocialAuthStrategy 与 XcxAuthStrategy 都不调用 checkLogin。
// RetryCountMsg / RetryExceedMsg 对空键返回空串，调用方须判空后再决定是否提示。
//
// LoginTypeSocial 是本项目新增：Java 的 LoginType 只有 4 个值，而 grantType
// 实际有 5 种（password/sms/email/social/xcx，见 sys_client.grant_type 与各
// @Service("xxx"+BASE_NAME)）。Java 能少一个是因为 social 不需要文案、
// 也从不传进 checkLogin；Go 侧 Code 是查表键，缺一个会让 social 查不到，
// 故补齐以保持 Code 与 grantType 的取值域一致。
var (
	LoginTypePassword = LoginType{
		Code:           "password",
		RetryCountKey:  "user.password.retry.limit.count",
		RetryExceedKey: "user.password.retry.limit.exceed",
	}
	LoginTypeSms = LoginType{
		Code:           "sms",
		RetryCountKey:  "sms.code.retry.limit.count",
		RetryExceedKey: "sms.code.retry.limit.exceed",
	}
	LoginTypeEmail = LoginType{
		Code:           "email",
		RetryCountKey:  "email.code.retry.limit.count",
		RetryExceedKey: "email.code.retry.limit.exceed",
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
//
// ctx 决定用哪种语言（i18n 中间件已把请求语言写进去），取不到语言时
// i18n.Msg 会回落默认语言，不会失败。
func (t LoginType) RetryCountMsg(ctx context.Context, errCount int) string {
	if t.RetryCountKey == "" {
		return ""
	}
	return i18n.Msg(ctx, t.RetryCountKey, errCount)
}

// RetryExceedMsg 渲染「错误次数超限，账户锁定」提示，对应原项目
// MessageUtils.message(loginType.getRetryLimitExceed(), maxRetryCount, lockTime)。
// 该登录方式不做重试计数时返回空串。
func (t LoginType) RetryExceedMsg(ctx context.Context, maxRetry int, lockMinutes int) string {
	if t.RetryExceedKey == "" {
		return ""
	}
	return i18n.Msg(ctx, t.RetryExceedKey, maxRetry, lockMinutes)
}
