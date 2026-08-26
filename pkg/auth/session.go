package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
)

// TokenKeyPrefix 会话在 Redis 里的键前缀，完整键为 auth:token:<jwt>。
const TokenKeyPrefix = "auth:token:"

// ErrSessionNotFound 会话不存在。
var ErrSessionNotFound = errors.New("auth: 会话不存在")

// Session Redis 里存的会话。
type Session struct {
	User *authmodel.LoginUser `json:"user"`
	// ActiveTimeout 空闲超时（秒），<=0 表示不设过期。
	ActiveTimeout int64 `json:"activeTimeout"`
}

// SessionStore Redis 会话读写。
type SessionStore struct {
	rdb goredis.UniversalClient
}

// NewSessionStore 构造会话存储。
func NewSessionStore(rdb goredis.UniversalClient) *SessionStore {
	return &SessionStore{rdb: rdb}
}

// key 拼接会话的完整 Redis 键。
func (s *SessionStore) key(token string) string {
	return TokenKeyPrefix + token
}

// Save 写入会话，TTL 取 ActiveTimeout。
func (s *SessionStore) Save(ctx context.Context, token string, sess *Session) error {
	if token == "" {
		return errors.New("auth: token 不能为空")
	}
	if sess == nil || sess.User == nil {
		return errors.New("auth: 会话内容不能为空")
	}

	payload, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("auth: 序列化会话失败: %w", err)
	}

	var ttl time.Duration // 0 传给 go-redis 即「不设过期」
	if sess.ActiveTimeout > 0 {
		ttl = time.Duration(sess.ActiveTimeout) * time.Second
	}
	if err := s.rdb.Set(ctx, s.key(token), payload, ttl).Err(); err != nil {
		return fmt.Errorf("auth: 写入会话失败: %w", err)
	}
	return nil
}

// Load 读取会话，不存在时返回 ErrSessionNotFound。
func (s *SessionStore) Load(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}

	payload, err := s.rdb.Get(ctx, s.key(token)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("auth: 读取会话失败: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return nil, fmt.Errorf("%w: 反序列化失败: %v", ErrSessionNotFound, err)
	}
	if sess.User == nil {
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// Renew 滑动续期，把会话的 TTL 重置为 activeTimeout。
func (s *SessionStore) Renew(ctx context.Context, token string, activeTimeout int64) error {
	if token == "" || activeTimeout <= 0 {
		return nil
	}
	ttl := time.Duration(activeTimeout) * time.Second
	if err := s.rdb.Expire(ctx, s.key(token), ttl).Err(); err != nil {
		return fmt.Errorf("auth: 会话续期失败: %w", err)
	}
	return nil
}

// Delete 删除会话，用于登出与踢下线。
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.rdb.Del(ctx, s.key(token)).Err(); err != nil {
		return fmt.Errorf("auth: 删除会话失败: %w", err)
	}
	return nil
}
