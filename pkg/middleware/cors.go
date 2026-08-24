package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// CORS 跨域中间件，配置取自 config.Get()。
func CORS() gin.HandlerFunc {
	return CORSWithConfig(config.Get().Middleware.CORS)
}

// CORSWithConfig 跨域中间件。必须注册在 Auth 之前。
func CORSWithConfig(cfg config.CORS) gin.HandlerFunc {
	maxAge := strconv.FormatInt(int64(cfg.MaxAge().Seconds()), 10)
	exposed := strings.Join(cfg.ExposedHeaders, ", ")

	return func(c *gin.Context) {
		h := c.Writer.Header()

		// Vary 在判断是否跨域之前就加，非跨域请求也带。
		h.Add("Vary", "Origin")
		h.Add("Vary", "Access-Control-Request-Method")
		h.Add("Vary", "Access-Control-Request-Headers")

		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// 预检 = OPTIONS + 带 Access-Control-Request-Method。
		reqMethod := c.GetHeader("Access-Control-Request-Method")
		isPreflight := c.Request.Method == http.MethodOptions && reqMethod != ""

		method := c.Request.Method
		if isPreflight {
			method = reqMethod
		}
		reqHeaders := splitHeaderList(c.GetHeader("Access-Control-Request-Headers"))

		allowOrigin, ok := matchOrigin(cfg.AllowedOriginPatterns, origin)
		if !ok {
			rejectCORS(c)
			return
		}
		allowMethods, ok := matchMethod(cfg.AllowedMethods, method)
		if !ok {
			rejectCORS(c)
			return
		}
		allowHeaders, ok := matchHeaders(cfg.AllowedHeaders, reqHeaders)
		if !ok {
			rejectCORS(c)
			return
		}

		h.Set("Access-Control-Allow-Origin", allowOrigin)
		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		if exposed != "" {
			h.Set("Access-Control-Expose-Headers", exposed)
		}

		if !isPreflight {
			c.Next()
			return
		}

		// 以下三个头只在预检响应里有意义。
		h.Set("Access-Control-Allow-Methods", allowMethods)
		if allowHeaders != "" {
			h.Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if cfg.MaxAgeSeconds > 0 {
			h.Set("Access-Control-Max-Age", maxAge)
		}

		c.AbortWithStatus(http.StatusOK)
	}
}

// rejectCORS 拒绝不合规的跨域请求。
func rejectCORS(c *gin.Context) {
	c.AbortWithStatus(http.StatusForbidden)
	_, _ = c.Writer.WriteString("Invalid CORS request")
}

// matchOrigin 匹配来源，返回应回显的 Origin。
func matchOrigin(patterns []string, origin string) (string, bool) {
	for _, p := range patterns {
		if wildcardMatch(p, origin) {
			return origin, true
		}
	}
	return "", false
}

// matchMethod 校验请求方法，返回 Access-Control-Allow-Methods 的值。
func matchMethod(allowed []string, method string) (string, bool) {
	if containsAll(allowed) {
		return method, true
	}
	for _, m := range allowed {
		if strings.EqualFold(m, method) {
			return strings.Join(allowed, ", "), true
		}
	}
	return "", false
}

// matchHeaders 校验预检请求的头，返回 Access-Control-Allow-Headers 的值。
func matchHeaders(allowed []string, requested []string) (string, bool) {
	if len(requested) == 0 {
		return "", true
	}
	if containsAll(allowed) {
		return strings.Join(requested, ", "), true
	}
	for _, req := range requested {
		matched := false
		for _, a := range allowed {
			// 头名大小写不敏感（RFC 9110）。
			if strings.EqualFold(a, req) {
				matched = true
				break
			}
		}
		if !matched {
			return "", false
		}
	}
	return strings.Join(requested, ", "), true
}

// containsAll 判断白名单是否为通配 "*"。
func containsAll(list []string) bool {
	for _, v := range list {
		if v == "*" {
			return true
		}
	}
	return false
}

// splitHeaderList 拆分逗号分隔的头列表并去掉空白项。
func splitHeaderList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// wildcardMatch 用 * 通配匹配 origin，大小写不敏感。
func wildcardMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, s)
	}

	pattern, s = strings.ToLower(pattern), strings.ToLower(s)
	segments := strings.Split(pattern, "*")

	// 首段必须贴着开头，尾段必须贴着结尾。
	if !strings.HasPrefix(s, segments[0]) {
		return false
	}
	s = s[len(segments[0]):]

	last := segments[len(segments)-1]
	middle := segments[1 : len(segments)-1]

	for _, seg := range middle {
		if seg == "" {
			continue
		}
		i := strings.Index(s, seg)
		if i < 0 {
			return false
		}
		s = s[i+len(seg):]
	}
	return strings.HasSuffix(s, last)
}
