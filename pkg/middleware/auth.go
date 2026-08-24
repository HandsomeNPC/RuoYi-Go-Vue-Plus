package middleware

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/response"
)

// LoginUserKey 当前登录用户存进 gin.Context 的键名。
//
// 与 TraceIDKey / LocaleKey 同样用 camelCase 对齐前端与 Java 侧字段风格。
// handler 用 CurrentUser(c) 取；service / repository 层从 context.Context 取，
// 走 auth 相关的 UserFromContext —— 那条路径不必 import gin。
const LoginUserKey = "loginUser"

// loginUserCtxKey 存进 context.Context 的键。
//
// 私有空结构体，与 trace.go 的 traceIDCtxKey、i18n 的 localeCtxKey 同一套做法：
// 标准库明确要求自定义 key 类型以避免跨包撞键，且零内存开销。
type loginUserCtxKey struct{}

// 鉴权失败的提示文案，取自原项目
// ruoyi-common-satoken/…/satoken/handler/SaTokenExceptionHandler.java。
//
// **有意用常量而非 i18n 词条**：Java 侧这几句就是硬编码在 handler 里的中文
// （不走 MessageUtils），pkg/i18n 里也确实没有对应的键。凭空新增词条会让
// 「Go 侧的文案」与「原项目的文案」出现一份无从核对的差异 ——
// 而 pkg/i18n 的其余 54×2 条都有与 .properties 的交叉验证兜着。
//
// Java 侧按 NotLoginException 的 type 分了 5 句，这里只保留能真正区分的两句：
// BE_REPLACED（被顶下线）/ KICK_OUT（被踢下线）需要「为什么没了会话」这一层
// 信息，而本实现删会话时不留因由（登出、顶下线、踢下线都只是 DEL 一个键）。
// 阶段 3 做在线用户管理时若要区分，得在删键前额外写一条「下线原因」——
// 那时再加文案，不要现在就摆一句永远走不到的分支。
const (
	// msgNotLogin 无 token / 非法 token / clientid 不匹配。
	// 对应 NotLoginException 的 default 分支（含 clientid 不匹配的 -100）。
	msgNotLogin = "登录状态异常，请重新登录"
	// msgTokenExpired token 过期或会话已不存在。
	// 对应 TOKEN_TIMEOUT / TOKEN_FREEZE 两个 type。
	msgTokenExpired = "登录已过期，请重新登录"
	// msgNoPermission 客户端访问路径或 IP 白名单校验失败。
	// 对应 NotPermissionException / NotRoleException。
	msgNoPermission = "没有访问权限，请联系管理员授权"
)

// Auth 鉴权中间件，配置取自 config.Get()、Redis 取包级默认客户端。
//
// 因此**必须先 config.Load 与 redis.Init 再注册**，否则 Get() / Client() 会
// panic —— 刻意为之：启动期编排错误不该留到运行时才发现。
func Auth() gin.HandlerFunc {
	cfg := config.Get()
	return AuthWithConfig(cfg.Middleware.Auth, cfg.JWT.Secret, redis.Client())
}

// AuthWithConfig 鉴权中间件，对应原项目
// ruoyi-common-security/…/security/config/SecurityConfig.java:80-119
// 注册的 sa-token SaInterceptor。
//
// # 两道前置跳过，再四步校验
//
//	0a. 命中 cfg.Excludes（Ant 风格）        -> 放行，对应 excludePathPatterns
//	0b. 未命中任何已注册路由                  -> 放行，交给 NoRoute 落 404（见下）
//	 1. Authorization 头取 token 并验签       -> 失败 401
//	 2. 查 Redis 会话（登出/空闲超时即无）    -> 失败 401
//	 3. clientid 与 token 里的交叉比对        -> 失败 401
//	 4. 客户端访问路径 -> 客户端 IP 白名单     -> 失败 403
//
// 四步的顺序对齐 Java，不能调换：先确认「是谁」再判「能否访问」，
// 否则未登录的请求会先撞上 403 而非 401，前端的处理分支完全不同。
//
// # 为什么未注册的路径不鉴权
//
// Java 侧 SaInterceptor 虽然注册在 /**，但内部用
// `SaRouter.match(allUrlHandler.getUrls())` 先筛一道 ——
// AllUrlHandler 在启动时遍历 RequestMappingHandlerMapping 收集**所有已注册
// 路由**（{pathVariable} 替换成 *）。也就是说未注册的路径根本不进鉴权，
// 直接落 404 而非 401。
//
// Go 侧用 c.FullPath() == "" 判定同一件事：gin 在路由匹配阶段就填好了它，
// 未命中任何路由时为空串。比启动时枚举 engine.Routes() 再 Ant 匹配既简单
// 又精确（那还要处理 :param / *wildcard 的归一化）。
//
// 代价是暴露了「哪些路径存在」这一位信息（404 vs 401）。这是**对齐原项目的
// 有意选择**：前端与测试可能依赖「乱路径返 404」，而扫描器能从路由表拿到的
// 信息本来也不靠这个隐藏。
//
// # 注册位置
//
// 按 README 的顺序表挂在最后（I18n 之后）：
//
//	Recover → CORS → TraceID → ApiEncrypt → RepeatableBody → AccessLog → XSS → I18n → Auth
//
// 三条顺序约束：CORS 必须在前（预检不带 token，先鉴权会被 401，
// 浏览器拿不到跨域头）；I18n 必须在前（虽然本中间件的文案是常量，
// 但 handler 里的业务文案要走词条）；XSS 必须在前（本中间件读 clientid
// 查询串参数，排在 XSS 之后拿到的才是清洗过的值）。
//
// # 相对 Java 的有意偏差
//
// 见 pkg/middleware/README.md 第 9 节的偏差表。三条最要紧的：
// token 只从 header 取（不接受查询串与 cookie）；claims 缺 clientid 时
// 返 401 而非像 Java 那样 NPE 成 500；错误统一走 c.Error 由 Recover 渲染。
//
// secret 由调用方显式传入而非在这里调 config.Get()：**WithConfig 版本一律
// 只用传入的配置**（本包的一贯约定，见 README「配置怎么读到的」），
// 否则测试和不走 Load 的调用方会直接 panic。
func AuthWithConfig(cfg config.Auth, secret string, rdb goredis.UniversalClient) gin.HandlerFunc {
	header := cfg.Header
	if header == "" {
		header = config.TokenHeader
	}
	clientIDHeader := cfg.ClientIDHeader
	if clientIDHeader == "" {
		clientIDHeader = config.ClientIDHeader
	}
	// TokenPrefix 为空是**有意义的取值**（不使用前缀），不回落默认值 ——
	// 想发裸 token 的部署方需要能表达这件事，默认值由 config.defaultAuth 铺。
	prefix := cfg.TokenPrefix
	excludes := cfg.Excludes
	store := auth.NewSessionStore(rdb)

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 0a. 免鉴权名单。复用 path.go 的 Ant 匹配 —— 与 xss.excludeUrls
		// 共用同一套实现，正是为了让免过滤名单与免鉴权名单不会在边界行为上分叉。
		if MatchAnyPath(path, excludes) {
			c.Next()
			return
		}

		// 0b. 未命中任何已注册路由 -> 不鉴权，交给 NoRoute 落 404。
		if c.FullPath() == "" {
			c.Next()
			return
		}

		// 1. 取 token 并验签。
		token := auth.TrimTokenPrefix(c.GetHeader(header), prefix)
		if token == "" {
			abortUnauthorized(c, msgNotLogin, "未携带 token")
			return
		}
		claims, err := auth.Verify(token, secret)
		if err != nil {
			// 过期与非法要分开：两者的前端行为相同，但日志里必须能区分
			// 「正常用到过期」和「token 被篡改」。
			if errors.Is(err, auth.ErrTokenExpired) {
				abortUnauthorized(c, msgTokenExpired, "token 已过期")
				return
			}
			abortUnauthorized(c, msgNotLogin, "token 校验失败: "+err.Error())
			return
		}

		// 2. 查会话。这是 JWT 的撤销机制 —— 只验签不查会话，登出就形同虚设。
		sess, err := store.Load(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrSessionNotFound) {
				abortUnauthorized(c, msgTokenExpired, "会话不存在(已登出或空闲超时)")
				return
			}
			// Redis 故障不是登录态问题，交给 Recover 兜成系统异常并打日志 ——
			// 回 401 会让一次 Redis 抖动表现成「所有人被登出」，
			// 而日志里只有一片 401，看不出真正的原因。
			_ = c.Error(err)
			c.Abort()
			return
		}

		// 3. clientid 交叉比对：header 或 query 命中其一即可，
		// 对齐 Java 的 StringUtils.equalsAny(clientId, headerCid, paramCid)。
		//
		// claims 里没有 clientid 时视为不匹配（401）。Java 侧
		// StpUtil.getExtra(CLIENT_KEY).toString() 对这种 token 会 NPE 成 500
		// （SecurityConfig.java:100），那是 bug 不是可对齐的行为。
		if claims.ClientID == "" ||
			!matchesAny(claims.ClientID, c.GetHeader(clientIDHeader), c.Query(clientIDHeader)) {
			abortUnauthorized(c, msgNotLogin, "客户端ID与Token不匹配")
			return
		}

		// 4. 客户端访问规则：先路径白名单，再 IP 白名单。
		// 对应 SecurityConfig.validateClientAccessRules（:147-166）。
		if msg, ok := checkClientRules(c, claims, path); !ok {
			abortForbidden(c, msg)
			return
		}

		// 校验通过：滑动续期。
		//
		// **失败只打日志不中断**：校验已经过了，此刻拒绝请求等于因为一次
		// Redis 抖动把已登录用户挡在门外。代价只是这次没能延长空闲窗口。
		if err := store.Renew(c.Request.Context(), token, sess.ActiveTimeout); err != nil {
			log.Printf("[auth]%s 会话续期失败(不影响本次请求): %v", logTracePrefix(c), err)
		}

		// 把登录用户写进两处上下文，与 trace.go / i18n.go 同一套做法：
		// gin.Context 给 handler，request context 给 service / repository ——
		// 后者不必为了拿当前用户去 import gin（阶段 4.1 的数据权限要用）。
		//
		// Token 字段在签发时不知道自己的值（签名依赖全部 claims），
		// 这里补上，让 handler / service 能拿到当前 token（登出要用它删会话）。
		sess.User.Token = token
		c.Set(LoginUserKey, sess.User)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), loginUserCtxKey{}, sess.User))

		c.Next()
	}
}

// checkClientRules 按客户端配置校验接口访问路径与来源 IP。
//
// 对应 SecurityConfig.validateClientAccessRules。两条规则都是
// **空即不限制**（对齐 Java 的 StringUtils.isNotBlank 判断）——
// sys_client 里 access_path / ip_whitelist 允许为 null，种子数据的 pc 客户端
// 两者都为空，即不受限。
//
// 注意规则来自 **JWT claims 而非实时查库**，与 Java 一致：
// 改了客户端的规则不会影响已签发的 token，要等它过期。
// 这是有意保持的行为 —— 每请求查一次 sys_client 会把鉴权变成带 DB 依赖的
// 热路径，而原项目也接受这个延迟。
func checkClientRules(c *gin.Context, claims *auth.Claims, path string) (string, bool) {
	if rules := splitClientRules(claims.ClientAccessPath); len(rules) > 0 {
		if !MatchAnyPath(path, rules) {
			return msgNoPermission, false
		}
	}

	if rules := splitClientRules(claims.ClientIPWhitelist); len(rules) > 0 {
		if !MatchAnyIPRule(ClientIP(c.Request), rules) {
			return msgNoPermission, false
		}
	}
	return "", true
}

// splitClientRules 按 , ; CR LF 切分客户端规则串。
//
// 对应 Java 的 CLIENT_RULE_SEPARATOR_REGEX "[,;\r\n]+" + str2List(…, true, true)
// （切分后 trim、丢弃空段）。
//
// 用 strings.FieldsFunc 而非 regexp：这两个规则串每请求都要切一次，
// 正则在热路径上没必要；FieldsFunc 天然合并连续分隔符并丢弃空段，
// 与 "+" 量词加 ignoreEmpty 的行为一致。
func splitClientRules(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, isClientRuleSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isClientRuleSeparator 客户端规则串的分隔符。
func isClientRuleSeparator(r rune) bool {
	return r == ',' || r == ';' || r == '\r' || r == '\n'
}

// matchesAny 判断 want 是否等于 candidates 中任意一个非空值。
//
// 对应 Java 的 StringUtils.equalsAny。**空候选值必须跳过**：
// 请求没带 clientid 时 header 与 query 都是空串，若不跳过，
// 一个 clientid 为空的 token 就能匹配上任何不带该头的请求。
// 上层已经拦了 claims.ClientID == ""，这里是第二道。
func matchesAny(want string, candidates ...string) bool {
	if want == "" {
		return false
	}
	for _, c := range candidates {
		if c != "" && c == want {
			return true
		}
	}
	return false
}

// abortUnauthorized 以 401 业务码终止请求。
//
// 走 c.Error 而非自己写响应体：由最外层的 Recover 统一渲染成 response.R，
// HTTP 状态码恒 200、业务码放响应体 —— 对齐 recover.go 的两条硬约束
// （前端统一拦 body.code，改成真实 4xx 会让拦截器失效）。
//
// reason 只进日志不回前端：把「token 过期」和「签名不符」的区别告诉调用方
// 没有价值，但对排查有价值。
func abortUnauthorized(c *gin.Context, msg, reason string) {
	log.Printf("[auth]%s 请求地址'%s',认证失败: %s",
		logTracePrefix(c), c.Request.URL.Path, reason)
	_ = c.Error(errs.NewCode(response.CodeUnauthorized, msg))
	c.Abort()
}

// abortForbidden 以 403 业务码终止请求。
//
// 与 401 分开是因为语义不同：401 是「你是谁」没搞清，403 是「你是谁清楚了，
// 但不允许访问」。前端对两者的处理不同 —— 前者跳登录页，后者提示无权限。
func abortForbidden(c *gin.Context, msg string) {
	log.Printf("[auth]%s 请求地址'%s',客户端访问规则校验失败",
		logTracePrefix(c), c.Request.URL.Path)
	_ = c.Error(errs.NewCode(response.CodeForbidden, msg))
	c.Abort()
}

// CurrentUser 从 gin.Context 取当前登录用户，未登录时返回 nil。
//
// 给 handler 用的便捷函数。service / repository 层拿到的是 context.Context，
// 应用 UserFromContext —— 那条路径不必 import gin。
//
// 返回 nil 而非报错：调用方多半在免鉴权接口里做「登录了就带上用户信息」
// 这类可选处理，为此写一层错误分支不成比例。需要强制登录的接口本来就
// 挂在鉴权中间件后面，走到 handler 时 nil 是不可能的。
func CurrentUser(c *gin.Context) *auth.LoginUser {
	if c == nil {
		return nil
	}
	u, _ := c.Get(LoginUserKey)
	user, _ := u.(*auth.LoginUser)
	return user
}

// UserFromContext 从 context.Context 取当前登录用户，未登录时返回 nil。
//
// 给 service / repository 层用（阶段 4.1 的数据权限要按当前用户的部门/角色
// 拼 SQL 条件，那一层不该 import gin）。
//
// 也兼容直接传 *gin.Context：gin.Context 实现了 context.Context，
// 其 Value 对非 string 键会回落到 Request.Context()。
func UserFromContext(ctx context.Context) *auth.LoginUser {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(loginUserCtxKey{}).(*auth.LoginUser)
	return user
}

// NewUserContext 返回携带登录用户的子 context。
//
// 中间件自己走的是 WithContext 那条路径，本函数给**脱离请求**的场景用：
// 定时任务、消息消费里需要以某个用户身份执行时（阶段 5 的 job 模块）。
// 与 i18n.NewContext 同一个用意。
func NewUserContext(ctx context.Context, user *auth.LoginUser) context.Context {
	return context.WithValue(ctx, loginUserCtxKey{}, user)
}
