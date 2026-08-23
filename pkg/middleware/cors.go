package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// CORS 跨域中间件，配置取自 config.Get()，对应 ResourcesConfig.corsFilter（:73-86）。
func CORS() gin.HandlerFunc {
	return CORSWithConfig(config.Get().Middleware.CORS)
}

// CORSWithConfig 跨域中间件，行为对齐 Spring 的 CorsFilter + DefaultCorsProcessor。
//
// 必须注册在 Auth 之前：预检请求是 OPTIONS 且不带 token，
// 先过鉴权会被 401，浏览器拿不到跨域头，前端所有请求全挂。
//
// 三个和 Java 对齐的关键点：
//
//  1. 回显 Origin，不吐 "*"。allowCredentials=true 配 Origin: * 是浏览器
//     明令禁止的组合。Java 用 allowedOriginPatterns（Spring 特有，会把 *
//     解析后回显具体 Origin）绕开，Go 手写时必须自己回显。
//  2. 预检请求直接结束，不进后续中间件 —— 对齐 CorsFilter 里
//     isPreFlightRequest 就 return、不调 filterChain.doFilter 的写法。
//  3. Vary 三个头无条件加。因为响应随 Origin 变化，不加会让 CDN／代理
//     把 A 站点的跨域头缓存给 B 站点用。
func CORSWithConfig(cfg config.CORS) gin.HandlerFunc {
	maxAge := strconv.FormatInt(int64(cfg.MaxAge().Seconds()), 10)
	exposed := strings.Join(cfg.ExposedHeaders, ", ")

	return func(c *gin.Context) {
		h := c.Writer.Header()

		// 对齐 DefaultCorsProcessor.processRequest：Vary 在判断是否跨域**之前**
		// 就加，非跨域请求也带。这是正确的缓存语义，不是冗余。
		h.Add("Vary", "Origin")
		h.Add("Vary", "Access-Control-Request-Method")
		h.Add("Vary", "Access-Control-Request-Headers")

		origin := c.GetHeader("Origin")
		if origin == "" {
			// 非跨域请求（同源、或 curl/服务端调用），不加任何 CORS 头。
			c.Next()
			return
		}

		// 预检 = OPTIONS + 带 Access-Control-Request-Method，
		// 对齐 CorsUtils.isPreFlightRequest。只看 method 会把普通的
		// OPTIONS 探测请求误判成预检。
		reqMethod := c.GetHeader("Access-Control-Request-Method")
		isPreflight := c.Request.Method == http.MethodOptions && reqMethod != ""

		// 实际请求的方法就是它自己的 method，预检时取 ACRM 头。
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
		// 只有预检会带 ACRH，实际请求这里是空列表，恒通过。
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

		// 以下三个头只在预检响应里有意义，实际请求带上纯属浪费带宽。
		h.Set("Access-Control-Allow-Methods", allowMethods)
		if allowHeaders != "" {
			h.Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if cfg.MaxAgeSeconds > 0 {
			h.Set("Access-Control-Max-Age", maxAge)
		}

		// 预检到此为止，不透给业务路由。200 空响应对齐 Java：CorsFilter
		// 直接 return，响应状态维持 Spring 的默认 200。
		c.AbortWithStatus(http.StatusOK)
	}
}

// rejectCORS 拒绝不合规的跨域请求，对应 DefaultCorsProcessor.rejectRequest。
//
// 预检和实际请求都是同一套处理：403 + 固定文案，不带任何 CORS 头。
//
// 这里回真实 403 而非 response.Fail，是**有意**偏离「HTTP 状态码恒为 200」
// 那条约束：跨域校验失败发生在浏览器的 CORS 协议层，响应体被浏览器直接吞掉，
// 前端拦截器根本读不到 body.code，回 200 只会让人误以为请求成功了。
func rejectCORS(c *gin.Context) {
	c.AbortWithStatus(http.StatusForbidden)
	_, _ = c.Writer.WriteString("Invalid CORS request")
}

// matchOrigin 匹配来源，返回应回显的 Origin。
//
// 对应 CorsConfiguration.checkOrigin + allowedOriginPatterns 的语义：
// 命中就返回**请求带来的 origin**，而不是配置里的 pattern。
// 这正是 Java 能在 allowCredentials=true 下配 "*" 还能用的原因。
func matchOrigin(patterns []string, origin string) (string, bool) {
	for _, p := range patterns {
		if wildcardMatch(p, origin) {
			return origin, true
		}
	}
	return "", false
}

// matchMethod 校验请求方法，返回 Access-Control-Allow-Methods 的值。
//
// 配了 "*" 时回显请求的方法而非字面 "*"，对齐 checkHttpMethod：
// resolvedMethods 为 null(即 ALL) 时返回 singletonList(requestMethod)。
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
//
// 对应 checkHeaders：配了 "*" 就逐个回显请求头；
// 否则**任一**头不在白名单内即整体拒绝（不是过滤掉不合规的那几个）。
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
			// 头名大小写不敏感（RFC 9110），必须用 EqualFold 而非 ==。
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
//
// 浏览器发的 Access-Control-Request-Headers 形如
// "content-type, authorization"，逗号后带空格。
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

// wildcardMatch 用 * 通配匹配 origin，* 可匹配任意长度字符（含空）。
//
// 支持 "*"、"https://*.example.com"、"http://localhost:*" 这类写法，
// 覆盖 Spring OriginPattern 的常见用法。大小写不敏感：Origin 的
// scheme 与 host 按 RFC 6454 不区分大小写，浏览器也总是发小写。
//
// 没有引入正则：pattern 来自配置文件（非用户输入），逐段扫描已经够用，
// 且省掉把 * 转义成 .* 时的边界问题。
func wildcardMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, s)
	}

	pattern, s = strings.ToLower(pattern), strings.ToLower(s)
	segments := strings.Split(pattern, "*")

	// 首段必须贴着开头，尾段必须贴着结尾，否则 "*.example.com"
	// 会被 "evil.com/x.example.com.attacker.net" 之类蒙过去。
	if !strings.HasPrefix(s, segments[0]) {
		return false
	}
	s = s[len(segments[0]):]

	last := segments[len(segments)-1]
	middle := segments[1 : len(segments)-1]

	// 中间各段按出现顺序依次匹配，每段从上一段之后开始找。
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
