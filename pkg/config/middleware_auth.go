package config

import (
	"github.com/spf13/viper"
)

// TokenHeader 读取 token 的请求头名。
//
// 对应原项目 sa-token 的 token-name（application.yml:91，值为 Authorization）。
const TokenHeader = "Authorization"

// TokenPrefix token 值的前缀。
//
// 对应 sa-token 的 token-prefix（common-satoken.yml，值为 Bearer），
// 即请求头形如 `Authorization: Bearer eyJhbGci...`。
const TokenPrefix = "Bearer"

// ClientIDHeader 客户端标识的请求头/查询串参数名。
//
// 对应 Java LoginHelper.CLIENT_KEY（LoginHelper.java:43）。鉴权时用它与
// token 里的 clientid 交叉比对，防止 app 端签发的 token 被拿去访问 pc 端接口。
const ClientIDHeader = "clientid"

// defaultAuthExcludes 免鉴权路径，Ant 风格（语义见 pkg/middleware/path.go）。
//
// 前 11 条逐字对应原项目 security.excludes（application.yml:100-113）。
//
// 末尾的 /auth/** 是**相对 Java 的必要新增**：那边 AuthController 整个类挂了
// @SaIgnore 注解来免鉴权，Go 没有注解机制，只能进这份名单 —— 漏了它，
// 登录接口自己就需要 token，谁也登不进来。同理 Java 侧 @SaIgnore 的
// CaptchaController / IndexController 待对应接口迁移时再往这里加。
//
// 注意 /*.html 与 /**/*.html 两条都要有：前者匹配根下一层（/index.html），
// 后者匹配更深层级（/static/a/index.html），Ant 语义下 `*` 不跨路径分隔符，
// 少一条就漏一批。
var defaultAuthExcludes = []string{
	"/*.html",
	"/**/*.html",
	"/**/*.css",
	"/**/*.js",
	"/favicon.ico",
	"/error",
	"/*/api-docs",
	"/*/api-docs/**",
	"/warm-flow-ui/config",
	"/snail-chat/**",
	"/api/snail/chat/**",
	"/auth/**",
}

// Auth 鉴权中间件配置，对应原项目 SecurityConfig + sa-token 的相关项。
//
// 键路径是 middleware.auth.*，而非 Java 的顶层 security.excludes ——
// Go 侧全部中间件配置都收在 middleware 段下、一个中间件一个文件，
// 为一项破例会让「某个中间件的配置在哪」这件事需要逐个记。
//
// 与其余中间件一样**没有 enabled 开关**：注册即启用，要关掉就删
// register.go 里那一行（理由见 Middleware 的说明）。
type Auth struct {
	// Excludes 免鉴权路径，Ant 风格。为空表示用 defaultAuthExcludes。
	//
	// 这是**安全配置**：与 xss.excludeUrls 共用 pkg/middleware/path.go 的
	// 同一套匹配实现，正是为了让免过滤名单与免鉴权名单不会在边界行为上分叉。
	Excludes []string `mapstructure:"excludes"`

	// Header 读取 token 的请求头名，为空表示用 TokenHeader。
	Header string `mapstructure:"header"`

	// TokenPrefix token 前缀，为空表示不使用前缀（裸 token）。
	//
	// 注意「为空」在这里是有意义的取值而非「用默认值」—— 想去掉 Bearer
	// 前缀的部署方需要能表达这件事。默认值由 defaultAuth 铺给 viper，
	// 显式写 `tokenPrefix: ""` 才是关闭。
	TokenPrefix string `mapstructure:"tokenPrefix"`

	// ClientIDHeader 客户端标识的头名，为空表示用 ClientIDHeader。
	//
	// 同名的查询串参数也会被读取（对齐 Java 侧 header 或 param 二选一命中），
	// 因为它不是凭证、只是标识；token 本身则**只从 header 取**，
	// 详见 pkg/middleware/auth.go 的说明。
	ClientIDHeader string `mapstructure:"clientIdHeader"`
}

// defaultAuth 返回对齐原项目行为的默认配置。
func defaultAuth() Auth {
	return Auth{
		Excludes:       defaultAuthExcludes,
		Header:         TokenHeader,
		TokenPrefix:    TokenPrefix,
		ClientIDHeader: ClientIDHeader,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
func (a Auth) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.auth.excludes", a.Excludes)
	v.SetDefault("middleware.auth.header", a.Header)
	v.SetDefault("middleware.auth.tokenPrefix", a.TokenPrefix)
	v.SetDefault("middleware.auth.clientIdHeader", a.ClientIDHeader)
}

// validate 校验鉴权配置。
//
// 空 Excludes 是合法的（表示什么都不排除，即全部接口都要鉴权）——
// 那是收紧而非放宽，不该拦。TokenPrefix 空同理表示不用前缀。
//
// 唯独不允许 Excludes 里出现空串：MatchAnyPath 对空 pattern 的处理是
// 「只匹配空路径」，看起来无害，但它在配置文件里的形态是一条
// `- ""` 或一个笔误留下的空行，写的人多半以为自己排除了什么。
func (a Auth) validate() error {
	for _, p := range a.Excludes {
		if p == "" {
			return errInvalid("middleware.auth.excludes", "不能包含空路径")
		}
	}
	return nil
}
