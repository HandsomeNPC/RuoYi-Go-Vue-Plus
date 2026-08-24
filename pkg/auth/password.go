package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost bcrypt 的代价因子。
const bcryptCost = 10

// ErrPasswordMismatch 密码不匹配。
var ErrPasswordMismatch = errors.New("auth: 密码不匹配")

// HashPassword 生成密码哈希。
func HashPassword(password string) (string, error) {
	if len(password) > 72 {
		return "", errors.New("auth: 密码长度超过 bcrypt 上限(72 字节)")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: 生成密码哈希失败: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword 校验密码，不符返回 ErrPasswordMismatch，哈希格式非法返回其他错误。
func VerifyPassword(password, hashed string) error {
	if hashed == "" {
		return ErrPasswordMismatch
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	if err == nil {
		return nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrPasswordMismatch
	}
	return fmt.Errorf("auth: 密码哈希格式非法: %w", err)
}
