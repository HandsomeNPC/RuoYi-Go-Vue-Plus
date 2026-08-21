package middleware

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/rand/v2"

	"github.com/gin-gonic/gin"
)

// TraceIDHeader 链路 id 的请求/响应头名。
//
// 用 X-Request-Id 而非 W3C 的 traceparent：前者是 nginx（$request_id）、
// 各家网关和前端 axios 拦截器的既有约定，接入成本最低。
const TraceIDHeader = "X-Request-Id"

// TraceIDKey 存进 gin.Context 的键名。
//
// 用 camelCase 对齐前端与 Java 侧的字段风格，日志/响应里出现的都是 traceId。
const TraceIDKey = "traceId"

// traceIDCtxKey 存进 context.Context 的键。
//
// 用私有空结构体而非 string：标准库明确要求自定义 key 类型以避免
// 跨包撞键，且零内存开销。service / repository 层通过 TraceIDFrom
// 取值，不必依赖 gin。
type traceIDCtxKey struct{}

// traceIDMaxLength 允许沿用的入站 id 最大长度。
//
// 32 是本地生成的长度，64 给上游网关（nginx $request_id 是 32 位十六进制、
// 部分 APM 会拼上 span 段）留出余量。超长的一律丢弃重新生成 ——
// 这个值会进每一条日志和响应头，不能让调用方决定它有多大。
const traceIDMaxLength = 64

// TraceIDConfig 链路 id 中间件配置。
//
// 原项目没有对照物（全项目零 MDC.put、零 Sleuth，logback-plus.xml 里的
// %tid 是被注释掉的 SkyWalking 残留），这里是 Go 侧自主设计。
type TraceIDConfig struct {
	// Header 读写链路 id 的头名，默认 TraceIDHeader。
	Header string

	// TrustInbound 是否沿用入站请求头里已有的 id。
	//
	// 默认 true —— 上游 nginx / 网关 / 调用方已经生成过 id 时必须沿用，
	// 否则同一次调用在各进程里拿到不同 id，链路就断了。本项目是
	// 「多模块拆进程 + nginx 负载均衡」，auth 与 system 之间将来若有
	// HTTP 调用，也靠这个头串起来。
	//
	// 反过来，进程直接暴露在公网时应关掉：id 由调用方决定意味着
	// 它可以给一万个请求发同一个 id，把日志检索搅乱。
	TrustInbound bool
}

// DefaultTraceIDConfig 返回默认配置。
func DefaultTraceIDConfig() TraceIDConfig {
	return TraceIDConfig{
		Header:       TraceIDHeader,
		TrustInbound: true,
	}
}

// TraceID 链路 id 中间件，用默认配置。
func TraceID() gin.HandlerFunc {
	return TraceIDWithConfig(DefaultTraceIDConfig())
}

// TraceIDWithConfig 链路 id 中间件：取或生成 id，写进上下文与响应头。
//
// 注册在 CORS 之后、其余中间件之前（见 README「注册顺序」）：越靠前，
// 越多的日志能带上 id。放在 CORS 之后是因为跨域预检会被 CORS 就地终止，
// 那种请求不进业务、也不需要 id。
//
// 三件事的顺序不能调换：
//
//  1. 先写响应头，再 c.Next()。响应头必须在 handler 写 body 之前落定，
//     一旦 body 开始输出，header 已经发出去了，事后 Set 无声无效。
//  2. 同时存进 gin.Context 和 request 的 context.Context。前者给中间件和
//     handler（c.GetString），后者让 service / repository 层能从纯
//     context.Context 里取到，不必为了拿个 id 去 import gin。
//  3. 入站 id 必须校验后才使用，见 sanitizeTraceID。
//
// 前端要读到这个头，还需要它出现在 Access-Control-Expose-Headers 里 ——
// 跨域下 JS 默认只能读 CORS 安全清单里的几个头。DefaultCORSConfig
// 已经把它加进 ExposedHeaders，两处是配套的，改一处要想到另一处。
func TraceIDWithConfig(cfg TraceIDConfig) gin.HandlerFunc {
	header := cfg.Header
	if header == "" {
		header = TraceIDHeader
	}

	return func(c *gin.Context) {
		var id string
		if cfg.TrustInbound {
			id = sanitizeTraceID(c.GetHeader(header))
		}
		if id == "" {
			id = NewTraceID()
		}

		c.Set(TraceIDKey, id)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), traceIDCtxKey{}, id))

		// 无条件回写：调用方带来的 id 也要回显，这样它能确认服务端
		// 认下了哪个 id（被 sanitize 丢掉时回的是新生成的那个）。
		c.Writer.Header().Set(header, id)

		c.Next()
	}
}

// TraceIDFrom 从 context.Context 取链路 id，取不到返回空串。
//
// 给 service / repository 层用：它们拿到的是 context.Context 而非
// *gin.Context。也兼容直接传 *gin.Context —— gin.Context 实现了
// context.Context，其 Value 会回落到 Request.Context()。
//
// 取不到返回空串而不是 panic 或生成新 id：调用方多半在打日志，
// 不能因为少个 id 就把请求搞挂；而在这里生成新 id 只会造出一个
// 跟任何请求都对不上的假 id，比空串更难排查。
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(traceIDCtxKey{}).(string)
	return id
}

// logTracePrefix 返回日志用的 " [traceId]" 片段，无 id 时返回空串。
//
// 供 Recover 等中间件拼进日志行，把一次请求的所有日志串起来 ——
// 这正是原项目缺失的能力（它只能靠每条错误各自的 8 位错误编号）。
//
// 前置空格放在返回值里而非格式串里：没有 id 时（比如没挂 TraceID
// 中间件，或 panic 发生在它之前）日志里不会留下多余空格。
func logTracePrefix(c *gin.Context) string {
	if id := c.GetString(TraceIDKey); id != "" {
		return " [" + id + "]"
	}
	return ""
}

// NewTraceID 生成 32 位十六进制链路 id。
//
// 16 字节的长度与格式对齐 W3C Trace Context 的 trace-id，将来接
// OpenTelemetry / SkyWalking 不用换格式；也与 nginx $request_id 一致。
//
// 用 math/rand/v2 而非 crypto/rand：链路 id 只需要「不重复」，不需要
// 「不可预测」（它不是凭证，猜中也没用）。math/rand/v2 的全局函数每次
// 进程启动自动随机播种、并发安全且无锁，还省掉 crypto/rand 那条
// 几乎不可能触发却必须处理的 error 分支。
func NewTraceID() string {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], rand.Uint64())
	binary.BigEndian.PutUint64(buf[8:16], rand.Uint64())
	return hex.EncodeToString(buf[:])
}

// sanitizeTraceID 校验入站 id，不合规返回空串（交由调用方重新生成）。
//
// 这个值来自外部且会被写进响应头和每一条日志，必须当作不可信输入：
//
//   - 含 CR/LF 会造成响应头注入（伪造额外的头甚至第二个响应），
//     以及日志注入（伪造出一整行假日志行）。
//   - 超长会把日志和响应头撑爆，属于零成本的放大攻击。
//   - 非可见字符会破坏日志的可读性与检索。
//
// 采取白名单而非过滤掉坏字符：过滤会把两个不同的入站 id 折叠成同一个，
// 反而制造出对不上的链路。字符集取 [0-9A-Za-z_-]，覆盖十六进制、
// UUID（含横线）和 base64url 风格的 id，够用且不含任何有语法意义的字符。
func sanitizeTraceID(id string) string {
	if id == "" || len(id) > traceIDMaxLength {
		return ""
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= '0' && c <= '9',
			c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c == '-', c == '_':
		default:
			return ""
		}
	}
	return id
}
