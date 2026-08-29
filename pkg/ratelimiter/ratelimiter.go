// Package ratelimiter 接口限流的注解（装饰器）层，对照 sa-token-go 的注解形式
// （sagin.CheckPermission）、pkg/encrypt 的 ApiEncrypt 与 Java @RateLimiter + RateLimiterAspect。
//
// 初始化对照 encrypt.Init / redis.Init：ratelimiter.Init() 无参，设包级实例；
// 路由用包级 ratelimiter.RateLimiter(...) / KeyFunc(...)。
//
// 与 Java 的差异（都是有意为之）：
//   - Java 用 Redisson RRateLimiter（Lua 滑动窗口令牌池），Go 无等价库，
//     这里用自写 Lua 滑动窗口(ZSET)，单次 EVAL 原子完成，语义等价且无竞态。
//   - Java 的 key 用 SpEL(#phoneNumber) 动态取参，Go 无 SpEL，
//     改用闭包 KeyFunc(func(*gin.Context) string)，编译期安全、无反射。
//   - Java 的 hsetnx 会让 count/time 改动一天内不生效，此处每次按当前参数计算，
//     改配置立即生效（有意不复刻那个坑）。
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
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/redis"
)

// LimitType 限流类型，对照 Java LimitType 枚举。
type LimitType int

const (
	// LimitTypeDefault 默认策略，全局限流（所有调用方共用一个额度）。
	LimitTypeDefault LimitType = iota
	// LimitTypeIP 按请求方 IP 限流。
	LimitTypeIP
	// LimitTypeCluster 按实例限流（集群多后端实例各自独立额度）。
	LimitTypeCluster
)

// 注解默认值，对照 Java @RateLimiter 的 default。
const (
	defaultTime  = 60 * time.Second // 限流窗口
	defaultCount = 100              // 窗口内允许的次数
	// defaultTimeout 限流键的存活时间，对照 Java timeout() default 86400。
	// 键在窗口结束后本就无用，留长 TTL 只为兜底，实际取 max(window, timeout)。
	defaultTimeout = 24 * time.Hour
)

// msgKey 触发限流时的 i18n 词条，对照 Java message() default "{rate.limiter.message}"。
const msgKey = "rate.limiter.message"

// options 一条限流注解的配置。
type options struct {
	window    time.Duration
	count     int
	limitType LimitType
	timeout   time.Duration
	// keyFunc 动态维度取值，对照 Java 的 SpEL key()。nil 表示不加动态维度。
	keyFunc func(*gin.Context) string
	// msgCode 提示词条，对照 Java message()。
	msgCode string
}

// Option 限流注解的可选参数。
type Option func(*options)

// WithTime 设置限流窗口，对照 Java time()（Java 单位是秒）。
func WithTime(d time.Duration) Option {
	return func(o *options) { o.window = d }
}

// WithCount 设置窗口内允许的次数，对照 Java count()。
func WithCount(n int) Option {
	return func(o *options) { o.count = n }
}

// WithLimitType 设置限流类型，对照 Java limitType()。
func WithLimitType(t LimitType) Option {
	return func(o *options) { o.limitType = t }
}

// WithTimeout 设置限流键存活时间，对照 Java timeout()。
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithMessage 自定义提示词条(i18n code)，对照 Java message()。
func WithMessage(code string) Option {
	return func(o *options) { o.msgCode = code }
}

// newOptions 铺默认值再套用 opts。
func newOptions(opts ...Option) *options {
	o := &options{
		window:    defaultTime,
		count:     defaultCount,
		limitType: LimitTypeDefault,
		timeout:   defaultTimeout,
		msgCode:   msgKey,
	}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// Limiter 限流器，持有实例 ID 与可注入的 Redis 客户端。
type Limiter struct {
	// instanceID 本进程实例标识，仅 LimitTypeCluster 用到，
	// 对照 Java RedisUtils.getClient().getId()。
	instanceID string
	// rdb 为 nil 时取包级 redis.Client()；仅测试注入独立客户端时才置值。
	rdb *goredis.Client
}

// New 构造限流器。instanceID 为空时自动生成。
func New(instanceID string) *Limiter {
	if instanceID == "" {
		instanceID = newInstanceID()
	}
	return &Limiter{instanceID: instanceID}
}

// client 返回本实例使用的 Redis 客户端。
func (l *Limiter) client() *goredis.Client {
	if l.rdb != nil {
		return l.rdb
	}
	return redis.Client()
}

// 包级默认实例（对照 redis.defaultClient / encrypt.defaultCrypto）。
var (
	mu             sync.RWMutex
	defaultLimiter *Limiter
)

// Init 构造并设置包级默认实例。对照 encrypt.Init。
// 必须在 redis.Init 之后调用（限流计数存 Redis）。
//
// Java 侧无开关配置(RateLimiterConfig 无条件注册切面)，此处对齐，不引入配置项。
func Init() {
	l := New("")
	mu.Lock()
	defaultLimiter = l
	mu.Unlock()
	log.Printf("ratelimiter 已就绪: instanceID=%s", l.instanceID)
}

// get 返回包级默认实例，未调用 Init 会 panic。对照 encrypt.getCrypto。
func get() *Limiter {
	mu.RLock()
	l := defaultLimiter
	mu.RUnlock()
	if l == nil {
		panic("ratelimiter: 尚未初始化，请先调用 ratelimiter.Init")
	}
	return l
}

// RateLimiter 限流注解（包级，对照 sagin.CheckPermission / encrypt.ApiEncrypt）。
// 不带动态维度，按 LimitType 决定全局/按 IP/按实例。
//
// 用法：g.GET("/auth/code", ratelimiter.RateLimiter(
//
//	ratelimiter.WithTime(60*time.Second),
//	ratelimiter.WithCount(10),
//	ratelimiter.WithLimitType(ratelimiter.LimitTypeIP)), handler.Code)
func RateLimiter(opts ...Option) gin.HandlerFunc {
	return handler(newOptions(opts...))
}

// KeyFunc 带动态维度的限流注解，对照 Java @RateLimiter(key = "#phoneNumber")。
// fn 从请求里取维度值（如手机号、邮箱），返回空串表示该请求不带动态维度。
//
// 用法：g.GET("/sms/code", ratelimiter.KeyFunc(
//
//	func(c *gin.Context) string { return c.Query("phoneNumber") },
//	ratelimiter.WithTime(60*time.Second), ratelimiter.WithCount(1)), handler.SmsCode)
func KeyFunc(fn func(*gin.Context) string, opts ...Option) gin.HandlerFunc {
	o := newOptions(opts...)
	o.keyFunc = fn
	return handler(o)
}

// handler 生成限流中间件，对照 Java RateLimiterAspect.doBefore。
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
		// Redis 异常时放行：限流是保护措施，不该因为它自己挂了就阻断全部业务。
		// 对照 Java 把非 ServiceException 包成 RuntimeException 阻断——此处有意不同，
		// 选择可用性优先，异常已记日志便于排查。
		log.Printf("[ratelimiter] %s %s 限流判定异常,已放行: %v",
			c.Request.Method, c.Request.URL.Path, err)
		c.Next()
		return
	}
	if remain < 0 {
		log.Printf("[ratelimiter] %s %s 触发限流, 缓存key => '%s'",
			c.Request.Method, c.Request.URL.Path, key)
		// 对照 Java throw new ServiceException(message)：Code 传 0，
		// 由 middleware.Recover 回落成 200 + code 500。
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), o.msgCode), ""))
		c.Abort()
		return
	}
	c.Next()
}

// combineKey 组装限流缓存键，对照 Java RateLimiterAspect.getCombineKey。
//
// 形如 global:rate_limit:<请求路径>:[<IP>:|<实例ID>:]<动态维度>
// 与 Java 一致：动态维度为空时保留结尾冒号。
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
// ARGV[3] 窗口内允许次数  ARGV[4] 键存活时间(毫秒)  ARGV[5] 本次请求的唯一成员
//
// 返回剩余次数；-1 表示超限（对照 Java RedisUtils.rateLimiter 的 -1 语义）。
var slidingWindowScript = goredis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local ttl    = tonumber(ARGV[4])
local member = ARGV[5]

-- 清掉窗口外的记录：权限在 window 毫秒后自动归还，对照 Redisson 的过期归还语义。
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
