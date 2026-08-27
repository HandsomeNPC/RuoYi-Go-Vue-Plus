package middleware

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/rand/v2"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// TraceIDHeader 链路 id 的请求/响应头名。
const TraceIDHeader = config.TraceIDHeader

// TraceIDKey 存进 gin.Context 的键名。
const TraceIDKey = "traceId"

// traceIDCtxKey 存进 context.Context 的键。
type traceIDCtxKey struct{}

// traceIDMaxLength 允许沿用的入站 id 最大长度。
const traceIDMaxLength = 64

// TraceID 链路 id 中间件，配置取自 config.Get()。
func TraceID() gin.HandlerFunc {
	return TraceIDWithConfig(config.Get().TraceID)
}

// TraceIDWithConfig 链路 id 中间件：取或生成 id，写进上下文与响应头。必须注册在 CORS 之后。
func TraceIDWithConfig(cfg config.TraceIDConfig) gin.HandlerFunc {
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

		// 无条件回写，调用方能确认服务端认下了哪个 id。必须在 c.Next() 之前写。
		c.Writer.Header().Set(header, id)

		c.Next()
	}
}

// TraceIDFrom 从 context.Context 取链路 id，取不到返回空串。
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(traceIDCtxKey{}).(string)
	return id
}

// logTracePrefix 返回日志用的 " [traceId]" 片段，无 id 时返回空串。
func logTracePrefix(c *gin.Context) string {
	if id := c.GetString(TraceIDKey); id != "" {
		return " [" + id + "]"
	}
	return ""
}

// NewTraceID 生成 32 位十六进制链路 id。
func NewTraceID() string {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], rand.Uint64())
	binary.BigEndian.PutUint64(buf[8:16], rand.Uint64())
	return hex.EncodeToString(buf[:])
}

// sanitizeTraceID 校验入站 id，不合规返回空串。采用白名单而非过滤，避免把不同的入站 id 折叠成同一个。
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
