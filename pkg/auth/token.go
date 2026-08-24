package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signingMethod JWT 签名算法。
//
// HMAC-SHA256 对齐原项目 sa-token 的 jwt-secret-key（对称密钥）配置。
// 硬编码而非做成配置项：算法可配等于把「用哪个算法」交给部署方，
// 而这里唯一有意义的变化是换成非对称（RS256），那需要同时改密钥管理，
// 不是改一个字符串能完成的事。
var signingMethod = jwt.SigningMethodHS256

// 校验失败的错误。调用方用 errors.Is 判别，用于区分「过期」与「非法」——
// 前者的提示是「登录已过期，请重新登录」，后者是「登录状态异常」。
var (
	// ErrTokenInvalid token 非法：签名不符、格式错误、算法不对。
	ErrTokenInvalid = errors.New("auth: token 非法")
	// ErrTokenExpired token 已过绝对有效期（claims 里的 exp）。
	ErrTokenExpired = errors.New("auth: token 已过期")
)

// Sign 签发 JWT。
//
// ttl 是**绝对有效期**，来自 sys_client.timeout（种子数据为 7 天）。
// 到期后无论多活跃都必须重新登录 —— 与 Redis 会话的滑动 TTL 是两道独立的闸，
// 见 session.go 的说明。
//
// secret 为空直接报错而不是签出一个用空密钥签名的 token：那种 token 谁都能伪造，
// 而它看起来与正常 token 毫无区别。配置校验（config.JWT.validate）已经拦了一道，
// 这里是第二道 —— 绕开配置直接调用本包的路径同样不该有这个口子。
func Sign(claims *Claims, secret string, ttl time.Duration) (string, error) {
	if claims == nil {
		return "", errors.New("auth: claims 不能为空")
	}
	if secret == "" {
		return "", errors.New("auth: 签名密钥不能为空")
	}

	now := time.Now()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))

	token, err := jwt.NewWithClaims(signingMethod, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("auth: 签发 token 失败: %w", err)
	}
	return token, nil
}

// Verify 校验 JWT 并返回其载荷。
//
// # 必须限定算法
//
// jwt.WithValidMethods 把可接受的算法钉死成 HS256。**不加这一条就是一个
// 完整的鉴权绕过**：库默认信任 token 自己 header 里声明的 alg，
// 攻击者把 alg 改成 "none" 就能免签名，或改成 RS256 让服务端拿 HMAC 密钥
// 当公钥去验一个自己签的 token。这是 JWT 最经典的两个坑，
// 由 TestVerifyRejectsAlgNone 与 TestVerifyRejectsAlgConfusion 锁住。
//
// # 过期与非法要能区分
//
// 库把两者都归到 parse 的 error 里，这里拆开：前端对「过期」的处理是
// 跳登录页重新登录，对「非法」的处理也是跳登录页 —— 行为一样，
// 但提示文案不同（对齐 Java SaTokenExceptionHandler 的 switch）。
// 混成一句会让「token 被篡改」和「正常用到过期」在日志里无法区分。
func Verify(token, secret string) (*Claims, error) {
	if token == "" {
		return nil, ErrTokenInvalid
	}
	if secret == "" {
		return nil, errors.New("auth: 签名密钥不能为空")
	}

	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
	)
	if err != nil {
		// 过期是唯一需要单独识别的失败原因，其余（签名不符、算法不对、
		// 格式错误）一律折叠成「非法」—— 对外只有文案区别，对内细节进日志。
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	return &claims, nil
}

// TrimTokenPrefix 剥掉 token 的前缀（默认 "Bearer"）并返回裸 token。
//
// 对应 sa-token 的 token-prefix 配置（common-satoken.yml 里是 "Bearer"）。
// 请求头形如 `Authorization: Bearer eyJhbGci...`。
//
// 前缀比对**大小写不敏感**：RFC 6750 定义的是 "Bearer"，但实践中
// curl 手写、老前端发 "bearer" 的都有，为大小写让用户拿到 401 只会浪费排查时间。
// 分隔符只认空格，不接受 tab —— 那不是任何客户端会发出的形态。
//
// 无前缀时**原样返回**而非报错：prefix 配空串意味着不使用前缀，
// 且容忍前端直接发裸 token（sa-token 同样容忍）。
func TrimTokenPrefix(value, prefix string) string {
	value = strings.Trim(value, " ")
	if value == "" || prefix == "" {
		return value
	}
	// 必须连带后面那个空格一起比，否则前缀为 "Bearer" 时
	// 一个名为 "BearerXXX" 的 token 会被切成 "XXX"。
	if len(value) > len(prefix) &&
		strings.EqualFold(value[:len(prefix)], prefix) &&
		value[len(prefix)] == ' ' {
		return strings.Trim(value[len(prefix)+1:], " ")
	}
	return value
}
