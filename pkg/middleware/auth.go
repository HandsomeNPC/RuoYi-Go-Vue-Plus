package middleware

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/response"
)

// LoginUserKey 当前登录用户存进 gin.Context 的键名。
const LoginUserKey = "loginUser"

// loginUserCtxKey 存进 context.Context 的键。
type loginUserCtxKey struct{}

const (
	msgNotLogin     = "登录状态异常，请重新登录"
	msgTokenExpired = "登录已过期，请重新登录"
	msgNoPermission = "没有访问权限，请联系管理员授权"
)

// Auth 鉴权中间件，配置取自 config.Get()、Redis 取包级默认客户端。
func Auth() gin.HandlerFunc {
	cfg := config.Get()
	return AuthWithConfig(cfg.Middleware.Auth, cfg.JWT.Secret, redis.Client())
}

// AuthWithConfig 鉴权中间件。
func AuthWithConfig(cfg config.Auth, secret string, rdb goredis.UniversalClient) gin.HandlerFunc {
	header := cfg.Header
	if header == "" {
		header = config.TokenHeader
	}
	clientIDHeader := cfg.ClientIDHeader
	if clientIDHeader == "" {
		clientIDHeader = config.ClientIDHeader
	}
	prefix := cfg.TokenPrefix
	excludes := cfg.Excludes
	store := auth.NewSessionStore(rdb)

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if MatchAnyPath(path, excludes) {
			c.Next()
			return
		}

		if c.FullPath() == "" {
			c.Next()
			return
		}

		token := auth.TrimTokenPrefix(c.GetHeader(header), prefix)
		if token == "" {
			abortUnauthorized(c, msgNotLogin, "未携带 token")
			return
		}
		claims, err := auth.Verify(token, secret)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				abortUnauthorized(c, msgTokenExpired, "token 已过期")
				return
			}
			abortUnauthorized(c, msgNotLogin, "token 校验失败: "+err.Error())
			return
		}

		sess, err := store.Load(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrSessionNotFound) {
				abortUnauthorized(c, msgTokenExpired, "会话不存在(已登出或空闲超时)")
				return
			}
			_ = c.Error(err)
			c.Abort()
			return
		}

		if claims.ClientID == "" ||
			!matchesAny(claims.ClientID, c.GetHeader(clientIDHeader), c.Query(clientIDHeader)) {
			abortUnauthorized(c, msgNotLogin, "客户端ID与Token不匹配")
			return
		}

		if msg, ok := checkClientRules(c, claims, path); !ok {
			abortForbidden(c, msg)
			return
		}

		if err := store.Renew(c.Request.Context(), token, sess.ActiveTimeout); err != nil {
			log.Printf("[auth]%s 会话续期失败(不影响本次请求): %v", logTracePrefix(c), err)
		}

		sess.User.Token = token
		c.Set(LoginUserKey, sess.User)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), loginUserCtxKey{}, sess.User))

		c.Next()
	}
}

// checkClientRules 按客户端配置校验接口访问路径与来源 IP。
func checkClientRules(c *gin.Context, claims *auth.Claims, path string) (string, bool) {
	if rules := splitClientRules(claims.ClientAccessPath); len(rules) > 0 {
		if !MatchAnyPath(path, rules) {
			return msgNoPermission, false
		}
	}

	if rules := splitClientRules(claims.ClientIPWhitelist); len(rules) > 0 {
		if !MatchAnyIPRule(ip.ClientIP(c.Request), rules) {
			return msgNoPermission, false
		}
	}
	return "", true
}

// splitClientRules 按 , ; CR LF 切分客户端规则串。
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
func abortUnauthorized(c *gin.Context, msg, reason string) {
	log.Printf("[auth]%s 请求地址'%s',认证失败: %s",
		logTracePrefix(c), c.Request.URL.Path, reason)
	_ = c.Error(errs.New(response.CodeUnauthorized, msg, ""))
	c.Abort()
}

// abortForbidden 以 403 业务码终止请求。
func abortForbidden(c *gin.Context, msg string) {
	log.Printf("[auth]%s 请求地址'%s',客户端访问规则校验失败",
		logTracePrefix(c), c.Request.URL.Path)
	_ = c.Error(errs.New(response.CodeForbidden, msg, ""))
	c.Abort()
}

// CurrentUser 从 gin.Context 取当前登录用户，未登录时返回 nil。
func CurrentUser(c *gin.Context) *authmodel.LoginUser {
	if c == nil {
		return nil
	}
	u, _ := c.Get(LoginUserKey)
	user, _ := u.(*authmodel.LoginUser)
	return user
}

// UserFromContext 从 context.Context 取当前登录用户，未登录时返回 nil。
func UserFromContext(ctx context.Context) *authmodel.LoginUser {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(loginUserCtxKey{}).(*authmodel.LoginUser)
	return user
}

// NewUserContext 返回携带登录用户的子 context。
func NewUserContext(ctx context.Context, user *authmodel.LoginUser) context.Context {
	return context.WithValue(ctx, loginUserCtxKey{}, user)
}

// IsMatchIPRule 判断客户端 IP 是否命中单条规则。
func IsMatchIPRule(rule, clientIP string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" || clientIP == "" {
		return false
	}

	if rule == clientIP {
		return true
	}
	if strings.Contains(rule, "/") {
		return matchCIDR(rule, clientIP)
	}
	if strings.ContainsAny(rule, "*?") {
		return matchIPGlob(rule, clientIP)
	}
	return false
}

// MatchAnyIPRule 判断客户端 IP 是否命中任意一条规则。
func MatchAnyIPRule(clientIP string, rules []string) bool {
	for _, rule := range rules {
		if IsMatchIPRule(rule, clientIP) {
			return true
		}
	}
	return false
}

// matchCIDR 按 CIDR 网段匹配。
func matchCIDR(rule, clientIP string) bool {
	_, network, err := net.ParseCIDR(rule)
	if err != nil {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// To4() 非 nil 即为 IPv4。两边必须同族。
	if (network.IP.To4() != nil) != (ip.To4() != nil) {
		return false
	}
	return network.Contains(ip)
}

// matchIPGlob 按通配符匹配。
func matchIPGlob(rule, clientIP string) bool {
	return matchSegment(rule, clientIP)
}
