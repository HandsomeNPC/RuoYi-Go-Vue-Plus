package middleware

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/constant"
)

// 请求参数脱敏与截断。AccessLog 与 pkg/oplog 共用：两者都要把入参压成一行文本落日志，
// 都必须先抹掉密码类字段，也都踩同一个雪花 id 精度坑，故实现只保留一份。

// truncatedSuffix 截断标记。
const truncatedSuffix = "...(已截断)"

// maxSanitizeSize 结构化脱敏的体积上限，超过走原文兜底路径。
const maxSanitizeSize = 256 << 10

// msgSensitiveParamOmitted 参数无法结构化脱敏且疑似含敏感字段时的占位文案。
const msgSensitiveParamOmitted = "<参数无法解析且疑似含敏感字段，已省略>"

// SanitizeJSONParam 把 JSON 请求体脱敏并截断成一行文本，空体返回空串。
// exclude 是在 constant.ExcludeProperties 之外额外要剔除的字段名。
func SanitizeJSONParam(body []byte, maxLen int, exclude []string) string {
	if len(body) == 0 {
		return ""
	}
	fields := excludeFields(exclude)
	if len(body) > maxSanitizeSize {
		return rawParamLog(body, maxLen, fields)
	}

	// UseNumber 不能省：默认把数字解成 float64，会抹掉雪花 id 的尾数。
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var node any
	if err := dec.Decode(&node); err != nil {
		// 解析失败不影响主请求，退回原文。
		return rawParamLog(body, maxLen, fields)
	}

	removeSensitiveFields(node, fields)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 关掉 HTML 转义，日志是给人看的。
	enc.SetEscapeHTML(false)
	if err := enc.Encode(node); err != nil {
		return rawParamLog(body, maxLen, fields)
	}

	// Encode 会补一个换行符，留着会把一行日志撑成两行。
	return limitParam(strings.TrimRight(buf.String(), "\n"), maxLen)
}

// SanitizeFormParam 把 query 与表单参数脱敏并截断成一行文本，无参数时返回空串。
func SanitizeFormParam(c *gin.Context, maxLen int, exclude []string) string {
	_ = c.Request.ParseForm()

	form := c.Request.Form
	if len(form) == 0 {
		return ""
	}

	// 复制一份再删：Form 是 handler 后面要用的数据，日志绝不能改动请求本身。
	safe := make(url.Values, len(form))
	for k, v := range form {
		safe[k] = v
	}
	for _, field := range excludeFields(exclude) {
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

// excludeFields 合并全局敏感字段与调用方追加的字段。
func excludeFields(extra []string) []string {
	if len(extra) == 0 {
		return constant.ExcludeProperties
	}
	fields := make([]string, 0, len(constant.ExcludeProperties)+len(extra))
	fields = append(fields, constant.ExcludeProperties...)
	return append(fields, extra...)
}

// removeSensitiveFields 递归删掉敏感字段。
func removeSensitiveFields(node any, fields []string) {
	switch n := node.(type) {
	case map[string]any:
		for _, field := range fields {
			delete(n, field)
		}
		for _, child := range n {
			removeSensitiveFields(child, fields)
		}
	case []any:
		for _, child := range n {
			removeSensitiveFields(child, fields)
		}
	}
}

// rawParamLog 结构化脱敏失败时的兜底：截断原文，但先确认里面没有敏感字段。
func rawParamLog(body []byte, maxLen int, fields []string) string {
	raw := limitParam(string(body), maxLen)
	lower := strings.ToLower(raw)
	for _, field := range fields {
		if strings.Contains(lower, strings.ToLower(field)) {
			return msgSensitiveParamOmitted
		}
	}
	return raw
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
