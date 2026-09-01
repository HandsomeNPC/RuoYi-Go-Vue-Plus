package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// AccessLog 请求日志中间件，配置取自 config.Get()。
func AccessLog() gin.HandlerFunc {
	return AccessLogWithConfig(config.Get().AccessLog)
}

// AccessLogWithConfig 请求日志中间件，必须注册在 RepeatableBody 之后。
func AccessLogWithConfig(cfg config.AccessLogConfig) gin.HandlerFunc {
	maxLen := cfg.MaxParamLength
	if maxLen <= 0 {
		maxLen = config.DefaultMaxParamLength
	}
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		// 用 URL.Path 而非 Request.RequestURI：后者带原始查询串，会把 ?password=xxx 原样写进日志。
		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			c.Next()
			return
		}

		logRequestStart(c, path, maxLen)

		start := time.Now()
		// 用 defer 而非 c.Next() 之后直接打，保证开始必有结束。
		defer func() {
			log.Printf("[access]%s 结束请求 => URL[%s %s],状态[%d],耗时:[%d]毫秒",
				logTracePrefix(c), c.Request.Method, path,
				c.Writer.Status(), time.Since(start).Milliseconds())
		}()

		c.Next()
	}
}

// logRequestStart 打印入参日志。
func logRequestStart(c *gin.Context, path string, maxLen int) {
	prefix, method := logTracePrefix(c), c.Request.Method

	// 按 content-type 分流，不看请求方法。
	if isJSONRequest(c) {
		log.Printf("[access]%s 开始请求 => URL[%s %s],参数类型[json],参数:[%s]",
			prefix, method, path, SanitizeJSONParam(BodyBytes(c), maxLen, nil))
		return
	}

	params := SanitizeFormParam(c, maxLen, nil)
	if params == "" {
		log.Printf("[access]%s 开始请求 => URL[%s %s],无参数", prefix, method, path)
		return
	}
	log.Printf("[access]%s 开始请求 => URL[%s %s],参数类型[param],参数:[%s]",
		prefix, method, path, params)
}

// isJSONRequest 判断是否 JSON 请求。前缀匹配且大小写不敏感。
func isJSONRequest(c *gin.Context) bool {
	return strings.HasPrefix(strings.ToLower(c.ContentType()), ContentTypeJSON)
}
