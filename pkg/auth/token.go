package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signingMethod JWT 签名算法。
var signingMethod = jwt.SigningMethodHS256

// 校验失败的错误。
var (
	// ErrTokenInvalid token 非法。
	ErrTokenInvalid = errors.New("auth: token 非法")
	// ErrTokenExpired token 已过绝对有效期。
	ErrTokenExpired = errors.New("auth: token 已过期")
)

// Sign 签发 JWT。
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
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	return &claims, nil
}

// TrimTokenPrefix 剥掉 token 的前缀（默认 "Bearer"）并返回裸 token。
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
