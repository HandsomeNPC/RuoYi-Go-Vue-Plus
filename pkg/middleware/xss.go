package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// reHTMLTag 匹配一个 HTML 标签，对应 hutool 的 HtmlUtil.RE_HTML_MARK
// （`hutool-http/…/http/HtmlUtil.java:26`），被 cleanHtmlTag 用来把标签替换成空串。
//
// 原值是三段或：
//
//	(<[^<]*?>)|(<[\s]*?/[^<]*?>)|(<[^<]*?/[\s]*?>)
//
// 后两段**完全被第一段吞掉**（第一段的 [^<]*? 已经能匹配 `/p`、`br/`、` /p` 这些内容），
// 实测 `</p>` `<br/>` `< /p>` `<br/ >` `<a href='x'>` `<` `<a<b>` 七个用例两者输出一致，
// 见 xss_test.go 的 TestCleanHTMLTagMatchesHutoolRegex（含随机用例交叉验证）。
// 只留第一段是为了让这条正则能读懂，行为无差异。
//
// 惰性量词 *? 必须保留：改成贪心的话 `<b>x</b>` 会被整段吃掉（连 x 一起），
// 而 hutool 的语义是「清除标签但保留标签内的内容」。
var reHTMLTag = regexp.MustCompile(`<[^<]*?>`)

// defaultXSSSkipMethods 跳过 XSS 清洗的请求方法，对齐 XssFilter.handleExcludeURL
// 里 `HttpMethod.GET.matches(method) || HttpMethod.DELETE.matches(method)` 的判断。
//
// 这两个方法按 REST 语义不携带需要落库的内容，跳过是为了不动查询串 ——
// 搜索关键词里出现 `<` 是正常输入，清洗会把它连同后面的字符一起吃掉。
//
// **但这留下了一个真实的缺口**：带 XSS 载荷的 GET 查询参数不经任何清洗。
// 原项目就是这样，本包对齐；缺口本身由输出侧兜住（见下方 XSSWithConfig 的说明）。
// 提取成配置项而非硬编码，是为了让需要收紧的进程能改配置而不必改这个文件。
var defaultXSSSkipMethods = []string{http.MethodGet, http.MethodDelete}

// XSSConfig XSS 清洗配置，对应 web/config/properties/XssProperties.java（前缀 xss）。
//
// 没有对应 xss.enabled 的字段：Java 用 @ConditionalOnProperty 决定要不要注册这个
// filter，Go 侧「注册」就是 router.go 里那一行 r.Use(XSS())，不写即关闭。
// 再加一个布尔开关只会造出「注册了但不生效」这种要翻两处配置才能确诊的状态。
type XSSConfig struct {
	// ExcludeURLs 跳过清洗的路径，Ant 风格（见 path.go）。
	//
	// 对应 xss.excludeUrls（application.yml:193-196），现为
	// /system/notice 与 /warm-flow/save-json —— 富文本公告和流程定义 JSON
	// 需要原样存标签，清洗会直接破坏内容。
	ExcludeURLs []string

	// SkipMethods 跳过清洗的请求方法，为空表示用 defaultXSSSkipMethods。
	SkipMethods []string
}

// DefaultXSSConfig 返回对齐原项目 yaml 的默认配置。
func DefaultXSSConfig() XSSConfig {
	return XSSConfig{
		ExcludeURLs: []string{"/system/notice", "/warm-flow/save-json"},
		SkipMethods: defaultXSSSkipMethods,
	}
}

// XSS 请求清洗中间件，用默认配置。
func XSS() gin.HandlerFunc {
	return XSSWithConfig(DefaultXSSConfig())
}

// XSSWithConfig 请求清洗中间件，对应原项目
// web/filter/XssFilter.java + web/filter/XssHttpServletRequestWrapper.java。
//
// 做的事就一件：把请求里的 HTML 标签剔掉（保留标签内的文字），覆盖三处入参 ——
// 查询串、表单字段、JSON 请求体。Java 靠包装 request 后覆写 getParameter /
// getParameterMap / getInputStream 实现，Go 里没有包装层，直接改
// c.Request 上对应的字段。
//
// # 先说清楚它不是什么
//
// 剔标签是**纵深防御的一层，不是主要防线**。它拦不住 `javascript:` 协议、
// 事件属性拼接、HTML 实体编码的载荷，也管不了从 DB 读出来再渲染的老数据。
// 真正的防线在输出侧（前端渲染时转义 / 后端返回 JSON 而非 HTML 片段）。
// 把它当唯一防线会得出「已经防了 XSS」这个错误结论 —— 上面 GET 跳过的缺口
// 就是明证：真靠它兜底的话，那个缺口早该是个事故了。
//
// # 注册位置
//
// 按 README 的顺序表挂在 AccessLog 之后：
//
//	Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n → Auth
//
// 两条约束：
//
//   - **必须在 RepeatableBody 之后**。JSON 体从 BodyBytes(c) 取，取不到就跳过
//     body 清洗，绝不回头读 c.Request.Body —— 那会把一次性的 body 吃掉，
//     handler 再绑参数就是空的（与 logger.go 同一条纪律）。
//   - **必须在所有读参数的中间件与 handler 之前**。gin 的 c.Query 会把
//     URL.Query() 的结果缓存进内部 queryCache，缓存建立之后再改 RawQuery
//     不会生效。当前链路里 XSS 之前没有任何一环读 gin 参数（AccessLog 走
//     c.Request.ParseForm，不碰 gin 的缓存），阶段 1 的 Auth 读 clientid
//     排在 XSS 之后 —— 拿到的是清洗后的值，符合预期。
//
// 相对 Java 的顺序差一处：Java 的 XssFilter（order = HIGHEST+1）跑在
// RepeatableFilter 之前，拦截器读到的是**清洗后**的 body；Go 侧日志在 XSS
// 之前，记的是**原始**报文。这是有意的 —— 日志的用途是排查和取证，
// 需要看到攻击者到底发了什么，清洗后的版本反而把证据擦掉了。
func XSSWithConfig(cfg XSSConfig) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(cfg.SkipMethods))
	methods := cfg.SkipMethods
	if len(methods) == 0 {
		methods = defaultXSSSkipMethods
	}
	for _, m := range methods {
		skip[strings.ToUpper(strings.TrimSpace(m))] = struct{}{}
	}
	excludes := cfg.ExcludeURLs

	return func(c *gin.Context) {
		if _, ok := skip[c.Request.Method]; ok {
			c.Next()
			return
		}
		// 对齐 handleExcludeURL：Java 用 getServletPath()（不含查询串），
		// 对应 Go 的 URL.Path。用 RequestURI 会把查询串带进匹配，
		// 于是 /system/notice?x=1 匹配不上 /system/notice 这条排除规则。
		if MatchAnyPath(c.Request.URL.Path, excludes) {
			c.Next()
			return
		}

		sanitizeQuery(c)
		sanitizeForm(c)
		sanitizeJSONBody(c)

		c.Next()
	}
}

// cleanHTMLTag 剔掉所有 HTML 标签但保留标签内的文字，对应 HtmlUtil.cleanHtmlTag。
//
// 不含 `<` 时直接返回：标签必然以 `<` 开头，绝大多数参数值走这条捷径，
// 省掉一次正则扫描（这个中间件对每个参数值都要调一次）。
func cleanHTMLTag(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return reHTMLTag.ReplaceAllString(s, "")
}

// cleanParamValue 清洗单个参数值，对应 wrapper 里 cleanHtmlTag(value).trim()。
//
// trim 是照抄 Java 的：它对每个参数值都做了 .trim()。这实际上会改动用户数据
// （前后空格一律丢掉），但改成保留就与原项目行为不一致了 ——
// 前端可能已经依赖服务端替它去空格，这里不擅自变更。
func cleanParamValue(s string) string {
	return strings.TrimSpace(cleanHTMLTag(s))
}

// sanitizeQuery 清洗查询串，改写 URL.RawQuery。
//
// Java 侧没有这一步的直接对应物 —— 它覆写 getParameter 一次拦下查询串和
// 表单字段两者。Go 里两者存放位置不同（URL.RawQuery / Request.PostForm），
// 得分别处理。
//
// 改 RawQuery 而非只改 Request.Form，是因为 gin 的 c.Query 和
// binding.Query 都是从 URL.Query() 现场解析的，不看 Form。
//
// 解析失败（RawQuery 里有非法的 % 转义）时保持原样：此时
// url.ParseQuery 返回的是**部分**结果，拿它重新编码会静默丢掉解析失败的参数，
// 让 handler 收到一个残缺的查询串 —— 比不清洗更难排查。
// 这种请求本身已经不合法，交给 handler 的绑定去报错。
func sanitizeQuery(c *gin.Context) {
	raw := c.Request.URL.RawQuery
	// 没有 `<` 就不可能有标签，连解析都不必做，绝大多数请求走这条捷径。
	// 但 `%` 也要放过去解析 —— 载荷多半是 percent 编码的（%3Cscript%3E），
	// 只看裸 `<` 会漏掉全部编码过的情况。
	if raw == "" || (!strings.Contains(raw, "<") && !strings.Contains(raw, "%")) {
		return
	}

	values, err := url.ParseQuery(raw)
	if err != nil {
		log.Printf("[xss]%s 请求地址'%s',查询串解析失败,跳过清洗: %v",
			logTracePrefix(c), c.Request.URL.Path, err)
		return
	}
	if !cleanValues(values) {
		return
	}
	// Encode 会重新做 percent 编码并按键排序。参数顺序变化对
	// URL.Query() / c.Query 无影响（它们都按键取值）。
	c.Request.URL.RawQuery = values.Encode()
}

// sanitizeForm 清洗表单字段，就地改写已解析的 Request.Form / Request.PostForm。
//
// 对应 wrapper 的 getParameterMap —— Java 那边是「取的时候清洗」，
// Go 这边是「进来的时候把解析结果改掉」。
//
// 就地改而非复制：这里的目的**就是**要让 handler 收到清洗后的值，
// 与 logger.go 里「必须复制一份再删」正好相反（那边是打日志，绝不能动请求）。
//
// multipart/form-data 有意不处理：ParseMultipartForm 会把上传的文件整个读进
// 内存（原项目允许 10MB 单文件），代价远超清洗几个文本字段的价值 ——
// 与 body.go 不缓存 multipart 是同一个取舍。此时只有查询串被清洗，
// 相比 Java 的 getParameterMap 会少掉 multipart 里的文本字段。
func sanitizeForm(c *gin.Context) {
	if strings.HasPrefix(strings.ToLower(c.ContentType()), "multipart/form-data") {
		return
	}

	// 主动 ParseForm 而不是「已解析才处理」：AccessLog 恰好会先调一次
	// ParseForm，但不能依赖它 —— 那个中间件可以被 SkipPaths 跳过、也可能
	// 根本没注册。不主动解析的话，handler 之后自己 ParseForm 拿到的就是
	// 未清洗的原始表单体（body 到那时还没被读过）。
	//
	// 出错不影响后续：Form 里仍有已解析出的部分，照常清洗；
	// 请求本身的合法性由 handler 的绑定去判。
	_ = c.Request.ParseForm()

	// 两个 map 都要清洗：Form 是查询串与表单体的并集（gin 的 c.PostForm
	// 在 formCache 未建时读 PostForm，binding.Form 读 Form），
	// 漏掉任一个都会让某条取值路径拿到未清洗的数据。
	cleanValues(c.Request.PostForm)
	cleanValues(c.Request.Form)
}

// cleanValues 就地清洗 url.Values 的所有值，返回是否发生了改动。
func cleanValues(values url.Values) bool {
	changed := false
	for _, vs := range values {
		for i, v := range vs {
			if cleaned := cleanParamValue(v); cleaned != v {
				vs[i] = cleaned
				changed = true
			}
		}
	}
	return changed
}

// sanitizeJSONBody 清洗 JSON 请求体，同时更新 c.Request.Body 与 gin 的 body 缓存。
//
// # 与 Java 的关键偏差：逐值清洗，不清洗整串
//
// Java 的 getInputStream 是 `HtmlUtil.cleanHtmlTag(整个 JSON 字符串)`，
// 这会**破坏 JSON 结构**。实测：
//
//	{"a":"1<2","b":"3>4"}  ->  {"a":"14"}
//
// 正则把 `<2","b":"3>` 整段当成一个标签吃掉了，`b` 字段凭空消失。
// 只要某个字符串值里有 `<`、后面任意位置还有 `>`，两者之间的所有内容
// （包括字段名、逗号、引号）就会被抹掉，handler 绑到的是一个结构被改过的对象。
// 这是原项目里一个实打实的 bug，不是可以「对齐」的行为。
//
// Go 侧改为：解析成树 → 只清洗字符串**值** → 序列化回去。结构不可能被破坏。
//
// 对象的**键**不清洗：键名带 HTML 标签的合法请求不存在，而清洗键会引入
// 一个更糟的失败模式 —— 两个不同的键清洗后撞成同一个，静默丢掉一个字段。
//
// 也不对值做 trim：Java 只对整个 JSON 串 trim 了一次（等于只动了首尾的空白，
// 对 JSON 语义无影响），并没有逐个 trim 值。这里保持一致，不擅自加。
//
// # 无效 JSON 不做处理
//
// 解析失败就原样放行，不退回去做字符串替换。这样的 body 必然过不了 handler
// 的 ShouldBindJSON，请求会被拒，载荷根本到不了落库那一步；
// 而对一段非法 JSON 做正则替换，除了制造上面那种结构损坏没有别的作用。
//
// # 有意不设体积上限
//
// logger.go 里有个 maxSanitizeSize（256KB），超限就走原文兜底 —— 那里能这么做，
// 是因为跳过的后果只是日志难看一点。这里**不能**：跳过清洗就是跳过一层防御，
// 攻击者只要把载荷填到阈值以上就能绕过。body 的总量已经由 RepeatableBody
// 的 MaxBodySize（10MB）兜住了，那才是该设限的地方。
func sanitizeJSONBody(c *gin.Context) {
	if !isJSONRequest(c) {
		return
	}
	// 取不到就跳过：非 JSON、body 为空、或没挂 RepeatableBody。
	// 绝不回头读 c.Request.Body（见 body.go 里 BodyBytes 的说明）。
	body := BodyBytes(c)
	if len(body) == 0 {
		return
	}
	// 整个 body 里没有 `<` 时不可能有标签，省掉一次完整的解析 + 序列化。
	// JSON 里的 `<` 无论出现在值、键还是字符串外，都必然是这个字节
	// （UTF-8 的多字节序列不含 ASCII 字节），所以这个判断不会漏。
	if !bytes.Contains(body, []byte("<")) {
		return
	}

	// UseNumber 不能省，与 logger.go 同一个理由：默认把数字解成 float64,
	// 只有 53 位有效位，19 位雪花 id 会被抹掉尾数。区别在于那边是打日志、
	// 打错了误导人，这边是**改请求** —— 精度丢了就是把 handler 要用的
	// 主键改成一个不存在的值，直接写坏数据。
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var node any
	if err := dec.Decode(&node); err != nil {
		return
	}

	cleaned, changed := cleanJSONNode(node)
	if !changed {
		return
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 关掉 HTML 转义：默认会把残留的 < > & 转成 < 这类序列。
	// JSON 解码后等价，但会让 body 无谓地膨胀，也让排查时对不上原文。
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cleaned); err != nil {
		// 编码失败时保持原 body 不动：宁可让未清洗的请求过去（后面还有
		// 输出侧转义兜着），也不能把一个残缺的 body 塞给 handler。
		log.Printf("[xss]%s 请求地址'%s',清洗后序列化失败,保持原请求体: %v",
			logTracePrefix(c), c.Request.URL.Path, err)
		return
	}
	// Encode 会补一个换行符。JSON 解析器不在乎，去掉是为了让
	// ContentLength 与 body 严格对上。
	out := bytes.TrimRight(buf.Bytes(), "\n")

	// 三处必须同时更新，漏一处就会让某条读取路径拿到未清洗的数据：
	//   Body            —— c.ShouldBindJSON 走这里
	//   gin.BodyBytesKey —— c.ShouldBindBodyWith 走这里（RepeatableBody 存的）
	//   ContentLength    —— 清洗后长度必然变短，不改会让读 body 的一方
	//                       按旧长度期待更多字节
	c.Request.Body = io.NopCloser(bytes.NewReader(out))
	c.Set(gin.BodyBytesKey, out)
	c.Request.ContentLength = int64(len(out))
}

// cleanJSONNode 递归清洗 JSON 树里的字符串值，返回清洗后的节点与是否有改动。
//
// map 与 slice 就地改（它们是引用类型，改了外层自然看得到），
// 但字符串是值类型，必须靠返回值往上传 —— 这也是这个函数要返回 any 的原因：
// 顶层 body 本身可以就是一个 JSON 字符串（`"<b>x</b>"` 是合法 JSON）。
//
// changed 一路往上冒是为了让调用方能在无改动时完全跳过序列化：
// 大多数请求即使含 `<`（比如富文本里的 `1 < 2`）也没有真标签，
// 这时重新序列化只会把字段顺序打乱、把数字格式改写，白费一次编码。
func cleanJSONNode(node any) (any, bool) {
	switch n := node.(type) {
	case string:
		cleaned := cleanHTMLTag(n)
		return cleaned, cleaned != n
	case map[string]any:
		changed := false
		for k, v := range n {
			if nv, c := cleanJSONNode(v); c {
				n[k] = nv
				changed = true
			}
		}
		return n, changed
	case []any:
		changed := false
		for i, v := range n {
			if nv, c := cleanJSONNode(v); c {
				n[i] = nv
				changed = true
			}
		}
		return n, changed
	default:
		// json.Number / bool / nil：不含字符串内容，无需清洗。
		return node, false
	}
}
