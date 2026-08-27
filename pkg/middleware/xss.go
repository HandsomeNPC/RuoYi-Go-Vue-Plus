package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// reHTMLTag 匹配一个 HTML 标签。
var reHTMLTag = regexp.MustCompile(`<[^<]*?>`)

// XSS 请求清洗中间件，配置取自 config.Get()。
func XSS() gin.HandlerFunc {
	return XSSWithConfig(config.Get().XSS)
}

// XSSWithConfig 请求清洗中间件：把请求里的 HTML 标签剔掉（保留标签内的文字）。
// 必须注册在 RepeatableBody 之后、所有读参数的中间件与 handler 之前。
func XSSWithConfig(cfg config.XSSConfig) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(cfg.SkipMethods))
	methods := cfg.SkipMethods
	if len(methods) == 0 {
		methods = config.DefaultXSS().SkipMethods
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
		// 用 URL.Path 而非 RequestURI，否则带查询串时匹配不上排除规则。
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

// cleanHTMLTag 剔掉所有 HTML 标签但保留标签内的文字。
func cleanHTMLTag(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return reHTMLTag.ReplaceAllString(s, "")
}

// cleanParamValue 清洗单个参数值。
func cleanParamValue(s string) string {
	return strings.TrimSpace(cleanHTMLTag(s))
}

// sanitizeQuery 清洗查询串，改写 URL.RawQuery。
func sanitizeQuery(c *gin.Context) {
	raw := c.Request.URL.RawQuery
	// 没有裸 `<` 也不含 `%` 时不可能有标签（载荷多半是 percent 编码的）。
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
	c.Request.URL.RawQuery = values.Encode()
}

// sanitizeForm 清洗表单字段，就地改写 Request.Form / Request.PostForm。
func sanitizeForm(c *gin.Context) {
	if strings.HasPrefix(strings.ToLower(c.ContentType()), "multipart/form-data") {
		return
	}

	// 主动 ParseForm，不能依赖 AccessLog 先调过它。
	_ = c.Request.ParseForm()

	// Form 是查询串与表单体的并集，两个 map 都要清洗。
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

// sanitizeJSONBody 清洗 JSON 请求体，逐值清洗字符串而不清洗整串，避免破坏 JSON 结构。
func sanitizeJSONBody(c *gin.Context) {
	if !isJSONRequest(c) {
		return
	}
	body := BodyBytes(c)
	if len(body) == 0 {
		return
	}
	// 整个 body 里没有 `<` 时不可能有标签。
	if !bytes.Contains(body, []byte("<")) {
		return
	}

	// UseNumber 不能省，否则 19 位雪花 id 会被 float64 抹掉尾数。
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
	// 关掉 HTML 转义，避免 body 无谓膨胀。
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cleaned); err != nil {
		// 编码失败时保持原 body 不动。
		log.Printf("[xss]%s 请求地址'%s',清洗后序列化失败,保持原请求体: %v",
			logTracePrefix(c), c.Request.URL.Path, err)
		return
	}
	// Encode 会补一个换行符，去掉让 ContentLength 与 body 严格对上。
	out := bytes.TrimRight(buf.Bytes(), "\n")

	// 三处必须同时更新：Body、gin.BodyBytesKey、ContentLength。
	c.Request.Body = io.NopCloser(bytes.NewReader(out))
	c.Set(gin.BodyBytesKey, out)
	c.Request.ContentLength = int64(len(out))
}

// cleanJSONNode 递归清洗 JSON 树里的字符串值，返回清洗后的节点与是否有改动。
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
		return node, false
	}
}
