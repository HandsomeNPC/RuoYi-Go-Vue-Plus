// Package repeatsubmit 防重复提交的注解（装饰器）层，对照 sa-token-go 的注解形式
// （sagin.CheckPermission）、pkg/encrypt 的 ApiEncrypt、pkg/ratelimiter 的 RateLimiter，
// 以及 Java @RepeatSubmit + RepeatSubmitAspect。
//
// 初始化对照 ratelimiter.Init / encrypt.Init：repeatsubmit.Init() 无参，设包级实例；
// 路由用包级 repeatsubmit.RepeatSubmit(...)。
//
// 判定逻辑对照 Java（参考美团 GTIS 防重）：
//   - 进 handler 前用 SETNX 抢锁，抢不到即判定重复提交并拦截；
//   - handler 成功（响应 code=200）则保留键，让 interval 内的重复请求都被挡住；
//   - handler 失败或 panic 则删除键，允许用户立刻重试。
//
// 与 Java 的差异（都是有意为之）：
//   - Java 用 AOP 取方法入参数组拼 JSON 做指纹，Go 无等价切面，
//     改用「请求体 + query 串」作指纹——这正是 HTTP 层等价的入参全集。
//   - Java 用 md5 拼指纹，此处用 sha256：该键只在本系统内自洽，不与 Java 侧互通，
//     没有兼容包袱，顺手避开 md5（gosec G401）。
//   - Java 在切面里抛 ServiceException 校验 interval < 1s，属运行期才暴露；
//     此处改为注册路由时 panic，启动即失败（对照 encrypt.Init 的 fail-fast）。
package repeatsubmit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/response"
)

// 注解默认值，对照 Java @RepeatSubmit 的 default。
const (
	// defaultInterval 间隔时间，小于此时间的重复请求视为重复提交。
	defaultInterval = 5000 * time.Millisecond
	// minInterval 允许的最小间隔，对照 Java "重复提交间隔时间不能小于'1'秒"。
	minInterval = time.Second
)

// msgKey 触发防重时的 i18n 词条，对照 Java message() default "{repeat.submit.message}"。
const msgKey = "repeat.submit.message"

// defaultTokenName 未配置时读取 token 的请求头名，对照 config.DefaultSAToken().TokenName。
const defaultTokenName = "Authorization"

// maxFingerprintBody 参与指纹计算的请求体上限。
// 超出部分不读：指纹只需足够区分两次提交，没必要为超大 body 付内存代价。
const maxFingerprintBody = 1 << 20 // 1MB

// options 一条防重注解的配置。
type options struct {
	// interval 间隔时间，对照 Java interval() + timeUnit()。
	interval time.Duration
	// msgCode 提示词条，对照 Java message()。
	msgCode string
}

// Option 防重注解的可选参数。
type Option func(*options)

// WithInterval 设置间隔时间，对照 Java interval() + timeUnit()。
// 小于 1 秒会在注册路由时 panic。
func WithInterval(d time.Duration) Option {
	return func(o *options) { o.interval = d }
}

// WithMessage 自定义提示词条(i18n code)，对照 Java message()。
func WithMessage(code string) Option {
	return func(o *options) { o.msgCode = code }
}

// newOptions 铺默认值再套用 opts，并校验 interval。
func newOptions(opts ...Option) *options {
	o := &options{
		interval: defaultInterval,
		msgCode:  msgKey,
	}
	for _, fn := range opts {
		fn(o)
	}
	// 对照 Java doBefore 的 interval < 1000 校验，但提前到注册期：
	// 路由是启动时装配的，此处 panic 等于配置错误就起不来，好过上线后才报错。
	if o.interval < minInterval {
		panic("repeatsubmit: 重复提交间隔时间不能小于 1 秒")
	}
	return o
}

// Submitter 防重提交器，持有 token 头名与可注入的 Redis 客户端。
type Submitter struct {
	// tokenName 读取 token 的请求头名，对照 Java SaManager.getConfig().getTokenName()。
	tokenName string
	// rdb 为 nil 时取包级 redis.Client()；仅测试注入独立客户端时才置值。
	rdb *goredis.Client
}

// New 构造防重提交器。tokenName 为空时取 defaultTokenName。
func New(tokenName string) *Submitter {
	if tokenName == "" {
		tokenName = defaultTokenName
	}
	return &Submitter{tokenName: tokenName}
}

// client 返回本实例使用的 Redis 客户端。
func (s *Submitter) client() *goredis.Client {
	if s.rdb != nil {
		return s.rdb
	}
	return redis.Client()
}

// 包级默认实例（对照 ratelimiter.defaultLimiter / encrypt.defaultCrypto）。
var (
	mu               sync.RWMutex
	defaultSubmitter *Submitter
)

// Init 按 config.Get().SAToken.TokenName 构造并设包级默认实例。对照 ratelimiter.Init。
// 必须在 config.Load 与 redis.Init 之后调用（防重键存 Redis）。
//
// Java 侧无开关配置(IdempotentConfig 无条件注册切面)，此处对齐，不引入配置项。
func Init() {
	c := config.Get()
	s := New(c.SAToken.TokenName)
	mu.Lock()
	defaultSubmitter = s
	mu.Unlock()
	log.Printf("[%s] repeatsubmit 已就绪: tokenName=%s", c.Server.Name, s.tokenName)
}

// get 返回包级默认实例，未调用 Init 会 panic。对照 ratelimiter.get。
func get() *Submitter {
	mu.RLock()
	s := defaultSubmitter
	mu.RUnlock()
	if s == nil {
		panic("repeatsubmit: 尚未初始化，请先调用 repeatsubmit.Init")
	}
	return s
}

// RepeatSubmit 防重复提交注解（包级，对照 sagin.CheckPermission / ratelimiter.RateLimiter）。
//
// 用法：g.POST("/user", repeatsubmit.RepeatSubmit(), handler.UserApiApp.Add)
// 自定义间隔：repeatsubmit.RepeatSubmit(repeatsubmit.WithInterval(10*time.Second))
//
// 注册顺序：须排在 encrypt.ApiEncrypt() 之后，这样取到的请求体是解密后的明文，
// 指纹才稳定（密文每次随机 AES 密钥，同样的入参会算出不同指纹，防重直接失效）。
func RepeatSubmit(opts ...Option) gin.HandlerFunc {
	o := newOptions(opts...)
	return func(c *gin.Context) {
		get().run(c, o)
	}
}

// run 执行一次防重判定，对照 Java RepeatSubmitAspect 的 doBefore / doAfterReturning / doAfterThrowing。
func (s *Submitter) run(c *gin.Context, o *options) {
	key := s.combineKey(c)

	locked, err := s.acquire(c.Request.Context(), key, o.interval)
	if err != nil {
		// Redis 异常时放行：防重是保护措施，不该因为它自己挂了就阻断全部业务。
		// 对照 pkg/ratelimiter 的同款取舍——可用性优先，异常已记日志便于排查。
		log.Printf("[repeatsubmit] %s %s 防重判定异常,已放行: %v",
			c.Request.Method, c.Request.URL.Path, err)
		c.Next()
		return
	}
	if !locked {
		log.Printf("[repeatsubmit] %s %s 触发防重提交, 缓存key => '%s'",
			c.Request.Method, c.Request.URL.Path, key)
		// 对照 Java throw new ServiceException(message)：Code 传 0，
		// 由 middleware.Recover 回落成 200 + code 500。
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), o.msgCode), ""))
		c.Abort()
		return
	}

	// 缓冲响应体：要读到业务 code 才能判定成功与否（Java 读的是 R.getCode()）。
	original := c.Writer
	buf := &bodyWriter{ResponseWriter: original}
	c.Writer = buf

	completed := false
	defer func() {
		// 还原 c.Writer，否则 Recover 在本函数返回后渲染错误会落进已用完的缓冲区。
		c.Writer = original
		if completed {
			return
		}
		// 走到这里说明 panic 正在展开，对照 Java doAfterThrowing：删键放行重试。
		// 缓冲区一律丢弃不 flush —— 一旦写出去 c.Writer.Written() 变 true，
		// Recover 就不会再渲染 500，客户端只能收到半截响应。
		s.release(c, key)
	}()

	c.Next()
	completed = true

	if s.succeeded(c, buf) {
		// 对照 Java：成功则不删 Redis 数据，保证 interval 内无法重复提交。
		buf.flush()
		return
	}
	// 失败：删键，让用户能立刻改参数重试。
	s.release(c, key)
	buf.flush()
}

// succeeded 判定本次请求是否成功，对照 Java doAfterReturning 里的 `r.getCode() == SUCCESS`。
//
// 三种情况判为失败：handler 登记了 c.Error、HTTP 状态非 2xx、响应体 code 非 200。
// 响应体解析不出 code（文件下载、非 R 结构等）时判为成功，对照 Java
// `if (jsonResult instanceof R<?> r)` 不匹配就什么都不做（即保留键）的语义。
func (s *Submitter) succeeded(c *gin.Context, buf *bodyWriter) bool {
	if len(c.Errors) > 0 {
		return false
	}
	if status := c.Writer.Status(); status < 200 || status > 299 {
		return false
	}
	code, ok := parseCode(buf.body.Bytes())
	if !ok {
		return true
	}
	return code == response.CodeSuccess
}

// acquire 抢占防重键，返回 true 表示抢到（本次请求放行）。
// 对照 Java RedisUtils.setObjectIfAbsent(key, "", Duration.ofMillis(interval))。
func (s *Submitter) acquire(ctx context.Context, key string, interval time.Duration) (bool, error) {
	return s.client().SetNX(ctx, key, "", interval).Result()
}

// release 删除防重键，对照 Java deleteRepeatKey。
func (s *Submitter) release(c *gin.Context, key string) {
	// 用 WithoutCancel：客户端断连会取消请求 context，但键该删还是得删，
	// 否则用户重连后要白等一个 interval 才能重试。
	ctx := context.WithoutCancel(c.Request.Context())
	if err := s.client().Del(ctx, key).Err(); err != nil {
		log.Printf("[repeatsubmit] %s %s 删除防重键失败, 缓存key => '%s': %v",
			c.Request.Method, c.Request.URL.Path, key, err)
	}
}

// combineKey 组装防重缓存键，对照 Java RepeatSubmitAspect.doBefore 的键拼装。
//
// 形如 global:repeat_submit:<请求路径><sha256(token + ":" + 入参)>
// 与 Java 一致：路径与指纹之间不加分隔符。
func (s *Submitter) combineKey(c *gin.Context) string {
	// 唯一值（没有消息头则退化为只按入参区分），对照 Java trimToEmpty(getHeader(tokenName))。
	token := s.token(c)

	sum := sha256.New()
	sum.Write([]byte(token))
	sum.Write([]byte(":"))
	sum.Write(requestParams(c))
	fingerprint := hex.EncodeToString(sum.Sum(nil))

	var sb strings.Builder
	sb.WriteString(constant.RepeatSubmitKey)
	sb.WriteString(c.Request.URL.Path)
	sb.WriteString(fingerprint)
	return sb.String()
}

// token 取当前请求的 token，作为防重的用户维度。
// 优先取 sagin.TokenInterceptor 写入 ctx 的值（已剥掉 Bearer 前缀、兼容 query/cookie/body 取值），
// 未挂拦截器时回落到直接读请求头。
func (s *Submitter) token(c *gin.Context) string {
	if t := strings.TrimSpace(sagin.GetTokenFromCtx(c)); t != "" {
		return t
	}
	return strings.TrimSpace(c.GetHeader(s.tokenName))
}

// requestParams 取参与指纹计算的入参，对照 Java argsArrayToString(point.getArgs())。
//
// Java 在切面里拿到的是方法入参数组；HTTP 层的等价物是「请求体 + query 串」，
// 二者都不带则指纹只由 token 与路径决定（此时同一用户对同一路径的连续裸请求会被防重，符合预期）。
func requestParams(c *gin.Context) []byte {
	query := c.Request.URL.RawQuery
	body := requestBody(c)

	if query == "" {
		return body
	}
	if len(body) == 0 {
		return []byte(query)
	}
	// 用空格分隔，对照 Java StringJoiner(" ")。
	buf := make([]byte, 0, len(body)+len(query)+1)
	buf = append(buf, body...)
	buf = append(buf, ' ')
	buf = append(buf, query...)
	return buf
}

// requestBody 取请求体。优先用 middleware.RepeatableBody 缓存的副本；
// 未缓存时自行读取并塞回 c.Request.Body，保证 handler 仍能读到完整 body。
func requestBody(c *gin.Context) []byte {
	if body := middleware.BodyBytes(c); body != nil {
		return body
	}
	if c.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxFingerprintBody))
	if err != nil {
		log.Printf("[repeatsubmit] %s %s 读取请求体失败,指纹降级为仅按路径与 token 计算: %v",
			c.Request.Method, c.Request.URL.Path, err)
		return nil
	}
	// 读完必须塞回去，否则下游 handler 拿到的是空 body。
	// 注意超过 maxFingerprintBody 的部分仍留在原 Body 里，用 MultiReader 接回来。
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), c.Request.Body))
	return body
}

// parseCode 从响应体里取业务 code，对照 Java 的 `jsonResult instanceof R<?>` 判定。
// 返回 ok=false 表示响应不是 R 结构（文件流、空响应等），调用方按成功处理。
func parseCode(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}
	// 只解 code 一个字段，避免为判定重复反序列化整个业务对象。
	var r struct {
		Code *int `json:"code"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Code == nil {
		return 0, false
	}
	return *r.Code, true
}

// bodyWriter 缓冲响应体的 gin.ResponseWriter 包装，对照 encrypt.cryptoWriter。
type bodyWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error)       { return w.body.Write(b) }
func (w *bodyWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }

// flush 把缓冲的响应体写进真正的响应。
func (w *bodyWriter) flush() {
	if w.body.Len() == 0 {
		// handler 没写响应（可能走了 c.Error 由 Recover 渲染），什么都不做。
		return
	}
	_, _ = w.ResponseWriter.Write(w.body.Bytes())
}
