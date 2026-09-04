package ratelimiter

import (
	"crypto/rand"
	"fmt"
	"time"
)

// newInstanceID 生成进程实例 ID，用于 LimitTypeCluster 的维度隔离。
func newInstanceID() string {
	id, err := randomHex(16)
	if err != nil {
		// 随机源失败几乎不可能；退化为时间戳，保证非空即可。
		return fmt.Sprintf("instance-%d", time.Now().UnixNano())
	}
	return id
}

// newMemberSuffix 生成滑动窗口 ZSET 成员的随机后缀，保证并发成员唯一。
func newMemberSuffix() string {
	b := make([]byte, 8)
	// 随机源极端失败时返回时间戳后缀，仍能保证这一刻唯一。
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// randomHex 生成 n 字节的十六进制随机串。
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
