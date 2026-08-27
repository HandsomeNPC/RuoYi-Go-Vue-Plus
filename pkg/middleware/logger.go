package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
)

// truncatedSuffix 截断标记。
const truncatedSuffix = "...(已截断)"

// maxSanitizeSize 结构化脱敏的体积上限，超过走原文兜底路径。
const maxSanitizeSize = 256 << 10

// msgSensitiveParamOmitted 参数无法结构化脱敏且疑似含敏感字段时的占位文案。
const msgSensitiveParamOmitted = "<参数无法解析且疑似含敏感字段，已省略>"

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
			prefix, method, path, jsonParamLog(BodyBytes(c), maxLen))
		return
	}

	params := queryParamLog(c, maxLen)
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

// jsonParamLog 把 JSON 请求体脱敏并截断成一行日志。
func jsonParamLog(body []byte, maxLen int) string {
	if len(body) == 0 {
		return ""
	}
	if len(body) > maxSanitizeSize {
		return rawParamLog(body, maxLen)
	}

	// UseNumber 不能省：默认把数字解成 float64，会抹掉雪花 id 的尾数。
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var node any
	if err := dec.Decode(&node); err != nil {
		// 解析失败不影响主请求，退回原文。
		return rawParamLog(body, maxLen)
	}

	removeSensitiveFields(node)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 关掉 HTML 转义，日志是给人看的。
	enc.SetEscapeHTML(false)
	if err := enc.Encode(node); err != nil {
		return rawParamLog(body, maxLen)
	}

	// Encode 会补一个换行符，留着会把一行日志撑成两行。
	return limitParam(strings.TrimRight(buf.String(), "\n"), maxLen)
}

// removeSensitiveFields 递归删掉敏感字段。
func removeSensitiveFields(node any) {
	switch n := node.(type) {
	case map[string]any:
		for _, field := range constant.ExcludeProperties {
			delete(n, field)
		}
		for _, child := range n {
			removeSensitiveFields(child)
		}
	case []any:
		for _, child := range n {
			removeSensitiveFields(child)
		}
	}
}

// rawParamLog 结构化脱敏失败时的兜底：截断原文，但先确认里面没有敏感字段。
func rawParamLog(body []byte, maxLen int) string {
	raw := limitParam(string(body), maxLen)
	lower := strings.ToLower(raw)
	for _, field := range constant.ExcludeProperties {
		if strings.Contains(lower, strings.ToLower(field)) {
			return msgSensitiveParamOmitted
		}
	}
	return raw
}

// queryParamLog 把请求参数脱敏并截断成一行日志，无参数时返回空串。
func queryParamLog(c *gin.Context, maxLen int) string {
	_ = c.Request.ParseForm()

	form := c.Request.Form
	if len(form) == 0 {
		return ""
	}

	// 复制一份再删：Form 是 handler 后面要用的数据，日志中间件绝不能改动请求本身。
	safe := make(url.Values, len(form))
	for k, v := range form {
		safe[k] = v
	}
	for _, field := range constant.ExcludeProperties {
		delete(safe, field)
	}
	if len(safe) == 0 {
		return ""
	}

	// 序列化的是 map<string, []string>，同名参数可出现多次。
	out, err := json.Marshal(safe)
	if err != nil {
		return ""
	}
	return limitParam(string(out), maxLen)
}

// limitParam 按字符数截断参数日志。
func limitParam(s string, maxLen int) string {
	// 一个 rune 至少占 1 字节，字节数没超就一定没超字符数。
	if len(s) <= maxLen {
		return s
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	count := 0
	for i := range s {
		if count == maxLen {
			return s[:i] + truncatedSuffix
		}
		count++
	}
	return s + truncatedSuffix
}
