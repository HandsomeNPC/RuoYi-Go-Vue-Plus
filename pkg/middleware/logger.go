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

	"ruoyi-go-vue-plus/pkg/constant"
)

// defaultMaxParamLength 参数日志的最大长度（字符数），对应
// PlusWebInvokeTimeInterceptor.MAX_PARAM_LOG_LENGTH = 4000。
const defaultMaxParamLength = 4000

// truncatedSuffix 截断标记。
//
// Java 侧直接 substring 不留痕迹，Go 这里有意加一个标记：
// 没有标记时，一段被砍断的 JSON 看起来就是一段非法 JSON，
// 排查的人会去追一个根本不存在的解析问题。
const truncatedSuffix = "...(已截断)"

// maxSanitizeSize 结构化脱敏的体积上限，超过就走原文兜底路径。
//
// Java 侧无此限制（readTree 整个 body）。Go 侧加它是因为解析结果
// 只为打日志，而日志最多留 4000 字符：为一个 10MB 的 body 建满树
// map/slice 再整体序列化，绝大部分成果会被立刻丢掉，纯属浪费 CPU 和 GC。
// 超限时走 rawParamLog，敏感字段仍然拦得住（见那里的说明）。
const maxSanitizeSize = 256 << 10

// msgSensitiveParamOmitted 参数无法结构化脱敏且疑似含敏感字段时的占位文案。
const msgSensitiveParamOmitted = "<参数无法解析且疑似含敏感字段，已省略>"

// AccessLogConfig 请求日志配置。
type AccessLogConfig struct {
	// MaxParamLength 参数日志最大字符数，<=0 表示用默认值 4000。
	//
	// 单位是字符（rune）不是字节，对齐 Java 的 substring 语义；
	// 按字节截会把一个中文字符劈成两半，日志里出现乱码。
	MaxParamLength int

	// SkipPaths 不打日志的路径（精确匹配 URL.Path）。
	//
	// Java 侧没有这个开关（拦截器注册在 /**）。Go 侧加它是给
	// 健康检查、探针这类高频且零信息量的路径用的 —— nginx / k8s
	// 每几秒探一次，不排除掉会把真正有用的日志冲走。
	// 默认为空，即行为与原项目一致。
	SkipPaths []string
}

// DefaultAccessLogConfig 返回默认配置，对齐原项目行为。
func DefaultAccessLogConfig() AccessLogConfig {
	return AccessLogConfig{
		MaxParamLength: defaultMaxParamLength,
		SkipPaths:      nil,
	}
}

// AccessLog 请求日志中间件，用默认配置。
func AccessLog() gin.HandlerFunc {
	return AccessLogWithConfig(DefaultAccessLogConfig())
}

// AccessLogWithConfig 请求日志中间件，对应原项目
// web/interceptor/PlusWebInvokeTimeInterceptor.java。
//
// 一次请求打两行：进入时打入参，结束时打耗时。对齐 Java 的
// preHandle / afterCompletion 两个钩子 —— 拆两行是刻意的，
// 只打一行的话，请求打到一半卡死或把进程搞挂时，日志里什么都不会留下。
//
// **必须注册在 RepeatableBody 之后**：JSON 入参从 BodyBytes(c) 取，
// 取不到就不打（打空串），绝不回头去读 c.Request.Body ——
// 那会把一次性的 body 吃掉，handler 再绑参数就是空的。
// 这正是 Java 侧 `if (request instanceof RepeatedlyRequestWrapper)`
// 的用意：宁可少打日志，也不能弄坏请求。
//
// 相比 Java 的三处偏差：
//
//   - 计时用闭包变量而非 ThreadLocal<StopWatch>。Go 的中间件天然是
//     每请求一个闭包，不需要也不该用「取出来—记得删」那套。
//   - 日志带 traceId 前缀。Java 侧「开始」「结束」两行在并发下根本
//     没法配对（全项目无 traceId，见 README），Go 侧靠它串起来。
//   - 结束行多打一个 HTTP 状态码。本项目业务码恒回 200，这个字段
//     平时确实没信息量，但它正好能暴露那几条不走业务码的路径：
//     CORS 拒绝的 403、未命中路由的 404/405。
func AccessLogWithConfig(cfg AccessLogConfig) gin.HandlerFunc {
	maxLen := cfg.MaxParamLength
	if maxLen <= 0 {
		maxLen = defaultMaxParamLength
	}
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		// 用 URL.Path 而非 Request.RequestURI：后者**带原始查询串**，
		// 直接打出去等于把 ?password=xxx 原样写进日志，
		// 下面对 query 做的脱敏就全白做了。查询参数走 queryParamLog。
		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			c.Next()
			return
		}

		logRequestStart(c, path, maxLen)

		start := time.Now()
		// 用 defer 而非 c.Next() 之后直接打：后续中间件或 handler
		// panic 时，栈会一路展开到最外层的 Recover，中途不经过这里。
		// 只有 defer 能保证「开始」必有「结束」—— 对齐 afterCompletion
		// 在 ex != null 时同样会被调用。
		defer func() {
			log.Printf("[access]%s 结束请求 => URL[%s %s],状态[%d],耗时:[%d]毫秒",
				logTracePrefix(c), c.Request.Method, path,
				c.Writer.Status(), time.Since(start).Milliseconds())
		}()

		c.Next()
	}
}

// logRequestStart 打印入参日志，对应 preHandle 的三个分支。
func logRequestStart(c *gin.Context, path string, maxLen int) {
	prefix, method := logTracePrefix(c), c.Request.Method

	// 按 content-type 分流，对齐 isJsonRequest —— 不看请求方法。
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

// isJSONRequest 判断是否 JSON 请求，对应 isJsonRequest。
//
// 前缀匹配且大小写不敏感：实际请求多半带参数
// （application/json;charset=UTF-8）。与 body.go 的缓存判定同源，
// 两处必须一致 —— 那边不缓存的，这边也就取不到 body。
func isJSONRequest(c *gin.Context) bool {
	return strings.HasPrefix(strings.ToLower(c.ContentType()), ContentTypeJSON)
}

// jsonParamLog 把 JSON 请求体脱敏并截断成一行日志。
//
// 对应 sanitizeJsonParam + limit：解析成树、递归删掉
// constant.ExcludeProperties 里的字段、再序列化回去。
// 用「解析后删字段」而非字符串替换，是因为后者会被嵌套结构、
// 转义引号和恰好同名的取值轻易绕过。
//
// body 为空（非 JSON 请求、没挂 RepeatableBody、body 本来就空）时返回空串，
// 对齐 Java 里 jsonParam 保持 "" 的分支 —— 照样打这行日志，只是参数为空。
func jsonParamLog(body []byte, maxLen int) string {
	if len(body) == 0 {
		return ""
	}
	if len(body) > maxSanitizeSize {
		return rawParamLog(body, maxLen)
	}

	// UseNumber 不能省：默认会把所有数字解成 float64，
	// 而本项目的主键是雪花 id（19 位），float64 只有 53 位有效位，
	// 打进日志的会是 1761100000000000000 这种被抹掉尾数的假值，
	// 拿去查库查不到，比不打更误导人。
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var node any
	if err := dec.Decode(&node); err != nil {
		// 对齐 Java：解析失败不影响主请求，退回原文。
		// 请求本身合不合法由 handler 的绑定去判，日志不掺和。
		return rawParamLog(body, maxLen)
	}

	removeSensitiveFields(node)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 关掉 HTML 转义：默认会把 < > & 转成 < 这类序列，
	// 日志是给人看的，富文本参数全变转义序列就没法读了。
	enc.SetEscapeHTML(false)
	if err := enc.Encode(node); err != nil {
		return rawParamLog(body, maxLen)
	}

	// Encode 会补一个换行符，留着会把一行日志撑成两行。
	return limitParam(strings.TrimRight(buf.String(), "\n"), maxLen)
}

// removeSensitiveFields 递归删掉敏感字段，对应 JsonUtils.removeFields。
//
// 对象删键后仍要继续下钻：敏感字段可能藏在数组元素或嵌套对象里
// （如 {"users":[{"password":"x"}]}）。
//
// 注意 Go 的 map 无序，序列化后的字段顺序与原文不同（Jackson 的
// ObjectNode 保留插入顺序）。日志可读性无损，不值得为此引入有序 map。
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
//
// 这里**有意偏离** Java —— sanitizeJsonParam 解析失败时直接返回原文，
// 于是一个非法 JSON（少个引号、被中断的请求）就能把明文密码原样带进日志。
// 日志的留存周期通常远长于会话，还会被采集到集中式平台，
// 这条路径在原项目里是一个实打实的凭证泄漏口子。
//
// 只按字段名做子串探测、命中就整体丢弃：既然连合法 JSON 都不是，
// 也就没有可靠的办法只摘掉那一个值，宁可整段不打。
// 探测在截断之后做 —— 我们只对真正会写进日志的那部分负责，
// 顺带避免对着一个 10MB 的 body 做全文扫描。
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
//
// 对应 preHandle 的 else 分支：getParameterMap → 删敏感键 → 转 JSON。
// Java 的 getParameterMap 同时含查询串和表单字段，所以这里走
// ParseForm 后的 Request.Form（两者的并集）而非 URL.Query()。
//
// ParseForm 不会弄坏后续绑定：它把结果缓存进 r.Form / r.PostForm，
// handler 再取参数拿的是解析结果而非 body，天然可重复
// （这也是 body.go 不缓存表单请求的原因）。表单体的读取上限由
// net/http 自己兜着（10MB），不必再加一道。
//
// multipart/form-data 有意不解析：那要把上传的文件整个读进内存，
// 代价远超一行日志的价值。此时只剩查询串，与 Java 相比会少掉表单字段。
func queryParamLog(c *gin.Context, maxLen int) string {
	// 出错时 Form 里仍有已解析出的查询串，照常打；
	// 参数本身有问题该由 handler 的绑定去报，不在日志里横插一杠。
	_ = c.Request.ParseForm()

	form := c.Request.Form
	if len(form) == 0 {
		return ""
	}

	// 复制一份再删：Form 是 handler 后面要用的数据，
	// 直接 delete 会让业务真的收不到这个参数 —— 日志中间件
	// 绝不能改动请求本身。
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

	// 与 Java 一致，序列化的是 map<string, []string>（值是数组），
	// 因为同名参数可以出现多次（?id=1&id=2）。
	out, err := json.Marshal(safe)
	if err != nil {
		return ""
	}
	return limitParam(string(out), maxLen)
}

// limitParam 按字符数截断参数日志，对应 limit(String)。
//
// 按 rune 而非 byte 截：中文参数从中间劈开会在日志里留下 U+FFFD 乱码，
// 更糟的是可能把后面的内容也带偏（终端按字节流解码）。
func limitParam(s string, maxLen int) string {
	// 一个 rune 至少占 1 字节，字节数没超就一定没超字符数 ——
	// 绝大多数请求走这条分支，省掉一次全串扫描。
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
