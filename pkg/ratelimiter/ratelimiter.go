// Package ratelimiter 接口限流注解（装饰器），对照 Java @RateLimiter + RateLimiterAspect。
//
// 与 Java 的有意差异：
//   - 滑动窗口用自写 Lua(ZSET) 单次 EVAL 原子完成（Go 无 Redisson 等价库）。
//   - 动态 key 用闭包 RateLimiterWithKeyFunc 替代 SpEL（Go 无 SpEL，编译期安全、无反射）。
//   - 每次按当前参数计算，count/time 改动立即生效（不复刻 Java hsetnx 缓存配置的坑）。
package ratelimiter

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/redis"
)

// LimitType 限流类型。
type LimitType int

const (
	LimitTypeGlobal  LimitType = iota // 全局限流（所有调用方共用一个额度）
	LimitTypeIP                       // 按请求方 IP 限流
	LimitTypeCluster                  // 按实例限流（多实例各自独立额度）
)

const (
	defaultTime    = 60 * time.Second // 默认限流窗口
	defaultCount   = 100              // 默认窗口内允许的次数
	defaultTimeout = 24 * time.Hour   // 键存活时间，实际取 max(window, timeout)
)

const defaultMessage = "访问过于频繁，请稍候再试" // 默认提示文案

type options struct {
	window    time.Duration             // 限流窗口
	count     int                       // 窗口内允许的次数
	limitType LimitType                 // 限流类型
	timeout   time.Duration             // 键存活时间
	keyFunc   func(*gin.Context) string // 动态维度取值，nil 表示不加
	message   string                    // 触发限流时的提示文案
}

// newOptions 构造限流配置。window/count/timeout 为零值、message 为空时取默认值。
func newOptions(window time.Duration, count int, limitType LimitType, timeout time.Duration, message string) *options {
	if window == 0 {
		window = defaultTime
	}
	if count == 0 {
		count = defaultCount
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if message == "" {
		message = defaultMessage
	}
	return &options{window: window, count: count, limitType: limitType, timeout: timeout, message: message}
}

// Limiter 限流器。
type Limiter struct {
	instanceID string          // 本进程实例标识，仅 LimitTypeCluster 用到
	rdb        *goredis.Client // 为 nil 时取包级 redis.Client()；仅测试注入
}

// client 返回本实例使用的 Redis 客户端。
func (l *Limiter) client() *goredis.Client {
	if l.rdb != nil {
		return l.rdb
	}
	return redis.Client()
}

var (
	mu             sync.RWMutex
	defaultLimiter *Limiter
)

// Init 构造包级默认实例，须在 redis.Init 之后调用（限流计数存 Redis）。
func Init() {
	l := &Limiter{instanceID: newInstanceID()}
	mu.Lock()
	defaultLimiter = l
	mu.Unlock()
	log.Printf("ratelimiter 已就绪: instanceID=%s", l.instanceID)
}

// get 返回包级默认实例，未 Init 会 panic。
func get() *Limiter {
	mu.RLock()
	l := defaultLimiter
	mu.RUnlock()
	if l == nil {
		panic("ratelimiter: 尚未初始化，请先调用 ratelimiter.Init")
	}
	return l
}

// RateLimiter 限流注解，按 limitType 决定全局/按 IP/按实例。零值取默认。
//
// 用法：ratelimiter.RateLimiter(60*time.Second, 10, ratelimiter.LimitTypeIP, 0, "")
func RateLimiter(window time.Duration, count int, limitType LimitType, timeout time.Duration, message string) gin.HandlerFunc {
	return handler(newOptions(window, count, limitType, timeout, message))
}

// RateLimiterWithKeyFunc 带动态维度的限流注解：fn 从请求里取维度值（如手机号），返回空串表示不带动态维度。零值取默认。
// 固定 LimitTypeGlobal：维度以 fn 的返回值为准，不再叠加 IP / 实例。
// fn 与 RateLimiter 的 limitType 同位，便于对照。
//
// 用法：ratelimiter.RateLimiterWithKeyFunc(60*time.Second, 1, func(c *gin.Context) string { return c.Query("phoneNumber") }, 0, "")
func RateLimiterWithKeyFunc(window time.Duration, count int, fn func(*gin.Context) string, timeout time.Duration, message string) gin.HandlerFunc {
	o := newOptions(window, count, LimitTypeGlobal, timeout, message)
	o.keyFunc = fn
	return handler(o)
}

// handler 生成限流中间件。
func handler(o *options) gin.HandlerFunc {
	return func(c *gin.Context) {
		get().run(c, o)
	}
}

// run 执行一次限流判定。
func (l *Limiter) run(c *gin.Context, o *options) {
	key := l.combineKey(c, o)
	remain, err := l.acquire(c.Request.Context(), key, o)
	if err != nil {
		// Redis 异常时放行：可用性优先（与 Java 阻断有意不同），异常已记日志。
		log.Printf("[ratelimiter] %s %s 限流判定异常,已放行: %v",
			c.Request.Method, c.Request.URL.Path, err)
		c.Next()
		return
	}
	if remain < 0 {
		log.Printf("[ratelimiter] %s %s 触发限流, 缓存key => '%s'",
			c.Request.Method, c.Request.URL.Path, key)
		_ = c.Error(errs.New(0, o.message, ""))
		c.Abort()
		return
	}
	c.Next()
}

// combineKey 组装限流缓存键：global:rate_limit:<路径>:[<IP>:|<实例ID>:]<动态维度>。
// 动态维度为空时保留结尾冒号。
func (l *Limiter) combineKey(c *gin.Context, o *options) string {
	var sb strings.Builder
	sb.WriteString(constant.RateLimitKey)
	sb.WriteString(c.Request.URL.Path)
	sb.WriteByte(':')

	switch o.limitType {
	case LimitTypeIP:
		sb.WriteString(ip.ClientIP(c.Request))
		sb.WriteByte(':')
	case LimitTypeCluster:
		sb.WriteString(l.instanceID)
		sb.WriteByte(':')
	}

	if o.keyFunc != nil {
		sb.WriteString(o.keyFunc(c))
	}
	return sb.String()
}

// slidingWindowScript 滑动窗口限流，单次 EVAL 原子完成，避免"读-判-写"竞态。
//
// KEYS[1] 限流键(ZSET)
// ARGV[1] 当前时间(毫秒)  ARGV[2] 窗口(毫秒)
// ARGV[3] 窗口内允许次数  ARGV[4] 键存活时间(毫秒)  ARGV[5] 本次请求唯一成员
// 返回剩余次数；-1 表示超限。
var slidingWindowScript = goredis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local ttl    = tonumber(ARGV[4])
local member = ARGV[5]

-- 清掉窗口外的记录：权限在 window 毫秒后自动归还。
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

local used = redis.call('ZCARD', key)
if used >= limit then
  -- 超限也要续期，避免键在持续打压下提前过期导致计数被重置。
  redis.call('PEXPIRE', key, ttl)
  return -1
end

redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, ttl)
return limit - used - 1
`)

// acquire 扣减一次额度，返回剩余次数；-1 表示超限。
func (l *Limiter) acquire(ctx context.Context, key string, o *options) (int64, error) {
	now := time.Now()
	// TTL 至少覆盖一个窗口，否则键会在窗口结束前消失导致限流失效。
	ttl := max(o.timeout, o.window)

	// 成员必须唯一，否则同一毫秒内的并发请求会因 ZADD 覆盖而只计一次。
	member := fmt.Sprintf("%d-%s", now.UnixNano(), newMemberSuffix())

	remain, err := slidingWindowScript.Run(ctx, l.client(), []string{key},
		now.UnixMilli(), o.window.Milliseconds(), o.count, ttl.Milliseconds(), member,
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("ratelimiter: 执行限流脚本失败: %w", err)
	}
	return remain, nil
}
