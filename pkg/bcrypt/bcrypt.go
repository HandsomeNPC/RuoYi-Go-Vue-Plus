// Package bcrypt bcrypt 密码哈希。
//
// hutool 的 BCrypt 暴露 gensalt/hashpw/checkpw 三件套，其中 gensalt 单独产出盐串。
// Go 标准库 golang.org/x/crypto/bcrypt 把盐折进哈希串里一次性返回，没有独立产盐的
// 公开 API；本包因此只对齐实际用到的高阶用法——hashpw(password) 自动产盐、
// checkpw(plaintext, hashed) 校验明文与哈希。
package bcrypt

import (
	"errors"
	"fmt"

	gobcrypt "golang.org/x/crypto/bcrypt"
)

// DefaultCost 默认代价因子。
const DefaultCost = 10

// MaxPasswordSize bcrypt 单次哈希的密码字节上限。
const MaxPasswordSize = 72

// ErrPasswordMismatch 密码与哈希不匹配。
var ErrPasswordMismatch = errors.New("bcrypt: 密码不匹配")

// ErrPasswordTooLong 密码超过 bcrypt 上限。
var ErrPasswordTooLong = errors.New("bcrypt: 密码长度超过上限(72 字节)")

// Hashpw 哈希明文密码，代价因子取 DefaultCost。
func Hashpw(password string) (string, error) {
	return HashpwWithCost(password, DefaultCost)
}

// HashpwWithCost 指定代价因子（4..31）哈希明文密码。
func HashpwWithCost(password string, cost int) (string, error) {
	if len(password) > MaxPasswordSize {
		return "", ErrPasswordTooLong
	}
	hashed, err := gobcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: 生成哈希失败: %w", err)
	}
	return string(hashed), nil
}

// Verify 校验密码：不匹配返回 ErrPasswordMismatch，哈希格式非法返回其他错误。
func Verify(plaintext, hashed string) error {
	if hashed == "" {
		return ErrPasswordMismatch
	}
	err := gobcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext))
	if err == nil {
		return nil
	}
	if errors.Is(err, gobcrypt.ErrMismatchedHashAndPassword) {
		return ErrPasswordMismatch
	}
	return fmt.Errorf("bcrypt: 哈希格式非法: %w", err)
}

// Checkpw 校验明文与哈希是否匹配。
func Checkpw(plaintext, hashed string) bool {
	return Verify(plaintext, hashed) == nil
}
