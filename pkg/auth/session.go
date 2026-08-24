package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// TokenKeyPrefix 会话在 Redis 里的键前缀，完整键为 auth:token:<jwt>。
//
// **有意不对齐 sa-token 的键布局**（satoken:login:token-session:<jwt> 等 4 个键）。
// 那套布局是框架内部约定，且值是 Jackson 带 @class 类型信息的多态序列化格式，
// 复刻它只有一个好处 —— 能与运行中的 Java 进程共用会话（灰度切流），
// 而本项目是重写不是双跑。用干净的布局换掉那份兼容负担。
//
// 另外两个前缀（在线用户、密码错误计数）已在 pkg/constant/cache_names.go，
// 那两个是**业务**键、原项目里也是自己拼的字符串，故沿用原名以保持数据可对照。
const TokenKeyPrefix = "auth:token:"

// ErrSessionNotFound 会话不存在：已登出、被踢下线，或已过空闲超时。
//
// 与 ErrTokenExpired 是两回事：那个是 JWT 的绝对超时（签发时就定了），
// 这个是服务端状态。token 没过期但会话没了，正是登出应有的表现 ——
// 如果只验 JWT 不查会话，登出就形同虚设（JWT 一经签发无法作废）。
var ErrSessionNotFound = errors.New("auth: 会话不存在")

// Session Redis 里存的会话。
//
// 除 LoginUser 外只多一个 ActiveTimeout，用途是滑动续期时知道续多久。
//
// **ActiveTimeout 存在这里而不是 JWT claims 里**：claims 的字段逐字对齐
// Java 的 8 个 extra（那是协议），而续期时长是服务端自己的实现细节。
// 放会话侧还有个实际好处 —— 改配置对**新会话**立即生效，
// 而 claims 一旦签出去就冻结了，改配置要等所有存量 token 过期才见效。
type Session struct {
	User *LoginUser `json:"user"`
	// ActiveTimeout 空闲超时（秒），来自 sys_client.active_timeout（默认 1800）。
	// <=0 表示不设过期（对应 sa-token 的 NEVER_EXPIRE = -1）。
	ActiveTimeout int64 `json:"activeTimeout"`
}

// SessionStore Redis 会话读写。
//
// 做成结构体而非包级函数：中间件在 r.Use(...) 那一刻拿到它并捕获进闭包，
// 每请求不再走 redis.Client() 的锁；测试也能塞一个指向 miniredis 的实例。
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
//
// 对应 Java LoginHelper.login 末尾的 StpUtil.getTokenSession().set(LOGIN_USER_KEY, loginUser)。
//
// ActiveTimeout <= 0 时不设过期，对齐 PlusSaTokenDao.writeValue 对
// NEVER_EXPIRE(-1) 的处理（那边 timeout==0 是静默丢弃，Go 侧把 0 与负数
// 一并当成「不过期」—— 静默丢弃会造出「登录成功但立刻就没会话」的状态，
// 那种行为不该被复刻）。
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
//
// 键不存在与反序列化失败都归到 ErrSessionNotFound：后者意味着 Redis 里
// 存着一份本进程读不懂的数据（跨版本、或键被别的东西占了），
// 对调用方而言同样是「没有可用会话」，让用户重新登录即可。
// 具体原因由调用方打日志区分。
func (s *SessionStore) Load(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}

	payload, err := s.rdb.Get(ctx, s.key(token)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrSessionNotFound
		}
		// Redis 连不上要与「会话不存在」区分开：前者是故障（应当 500 并告警），
		// 后者是正常的登录态过期（401）。混为一谈会让一次 Redis 抖动
		// 表现为「所有人被登出」，而日志里只有一片 401。
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
//
// 对应原项目 sa-token 的 active-timeout + dynamic-active-timeout: true ——
// 那边每请求更新 last-activity 键，Go 侧直接对会话键 EXPIRE，少一个键。
//
// activeTimeout <= 0 表示会话不过期，无需续期，直接返回。
//
// **续期失败不该让请求失败**，故调用方（中间件）只打日志不中断：
// 校验已经通过了，此刻拒绝请求等于因为一次 Redis 抖动把已登录用户挡在门外。
// 代价只是这次没能延长空闲窗口。
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
//
// 这是 JWT 唯一的作废手段 —— token 本身签发后不可撤销，
// 删掉会话让后续请求在第 2 步（查会话）被拦下。
//
// 删一个不存在的键不算错误（Redis DEL 返回 0），对齐登出的幂等语义：
// 重复登出、或 token 已过期后再登出，都不该报错。
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.rdb.Del(ctx, s.key(token)).Err(); err != nil {
		return fmt.Errorf("auth: 删除会话失败: %w", err)
	}
	return nil
}
