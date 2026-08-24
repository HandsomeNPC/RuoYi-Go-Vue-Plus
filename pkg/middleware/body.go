package middleware

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
)

// ContentTypeJSON JSON 请求的 content-type 前缀。
const ContentTypeJSON = config.ContentTypeJSON

// RepeatableBody 可重复读请求体中间件，配置取自 config.Get()。
func RepeatableBody() gin.HandlerFunc {
	return RepeatableBodyWithConfig(config.Get().Middleware.RepeatableBody)
}

// RepeatableBodyWithConfig 可重复读请求体中间件。
func RepeatableBodyWithConfig(cfg config.RepeatableBody) gin.HandlerFunc {
	maxSize := cfg.MaxBodySize
	if maxSize <= 0 {
		maxSize = config.DefaultMiddleware().RepeatableBody.MaxBodySize
	}
	types := make([]string, 0, len(cfg.ContentTypes))
	for _, t := range cfg.ContentTypes {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			types = append(types, t)
		}
	}

	return func(c *gin.Context) {
		if !shouldBufferBody(c, types) {
			c.Next()
			return
		}

		if c.Request.ContentLength > maxSize {
			rejectOversizedBody(c, c.Request.ContentLength, maxSize)
			return
		}

		// 多读 1 字节用来区分「刚好到上限」和「已经超了」。
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSize+1))
		if err != nil {
			log.Printf("[body]%s 请求地址'%s',读取请求体失败: %v",
				logTracePrefix(c), c.Request.URL.Path, err)
			_ = c.Error(err)
			c.Abort()
			return
		}
		if int64(len(body)) > maxSize {
			rejectOversizedBody(c, int64(len(body)), maxSize)
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Set(gin.BodyBytesKey, body)

		c.Next()
	}
}

// BodyBytes 取已缓存的请求体，未缓存时返回 nil。
func BodyBytes(c *gin.Context) []byte {
	v, ok := c.Get(gin.BodyBytesKey)
	if !ok {
		return nil
	}
	body, _ := v.([]byte)
	return body
}

// shouldBufferBody 判断本次请求是否需要缓存 body。
func shouldBufferBody(c *gin.Context, types []string) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return false
	}

	ct := strings.ToLower(c.ContentType())
	for _, t := range types {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}

// rejectOversizedBody 拒绝超限的请求体。
func rejectOversizedBody(c *gin.Context, size, maxSize int64) {
	log.Printf("[body]%s 请求地址'%s',请求体超限: %d > %d 字节",
		logTracePrefix(c), c.Request.URL.Path, size, maxSize)
	_ = c.Error(errs.New(0, "请求体超出大小限制", ""))
	c.Abort()
}
