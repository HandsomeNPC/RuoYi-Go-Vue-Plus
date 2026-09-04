package excel

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// contentType xlsx 的 MIME。
const contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// HeaderDownloadFilename 附件名响应头。
// 前端读它拿文件名，比解析 Content-Disposition 省事。
// 跨域下须在 cors.exposedHeaders 里放行，否则前端读不到。
const HeaderDownloadFilename = "download-filename"

// Export 把 rows 导出成 xlsx 附件写进响应。
//
// 先把整个工作簿建进内存再落笔，不是图省事：middleware.Recover 只在
// 响应还没被写过时才渲染错误，一旦抢先写了字节，后续错误会被静默吞掉，
// 客户端拿到的是 200 + 半截文件。所以所有可能失败的活都得在第一个字节之前干完。
func Export[T any](c *gin.Context, rows []T, sheetName string) error {
	buf, err := Write(rows, Options{SheetName: sheetName})
	if err != nil {
		return err
	}

	ascii, encoded := attachmentName(sheetName)
	h := c.Writer.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", attachment(ascii, encoded))
	h.Set(HeaderDownloadFilename, encoded)
	h.Set("Content-Length", strconv.Itoa(buf.Len()))
	// 附件名两个头都得放行到 CORS 之外，配置里漏了就地兜一层。
	h.Add("Access-Control-Expose-Headers", "Content-Disposition, "+HeaderDownloadFilename)

	c.Status(http.StatusOK)
	if _, err := c.Writer.Write(buf.Bytes()); err != nil {
		// 走到这里响应已开写，返回的错误只能进日志：
		// Recover 判定 Written() 后不会再渲染，也不该渲染——那会把 JSON 拼到文件尾。
		return fmt.Errorf("excel: 写响应失败: %w", err)
	}
	return nil
}

// attachment 拼 Content-Disposition。
// 同时给 filename 与 filename*：前者是纯 ASCII 兜底给老客户端，后者带 UTF-8 原名，
// 现代浏览器优先认后者。此处按 RFC 6266 写规范形态，
// 而非 filename 也塞百分号编码值、且 ; 后无空格——
// 那种写法非规范，只是靠 filename* 生效而侥幸能用。
func attachment(ascii, encoded string) string {
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, encoded)
}

// attachmentName 生成下载文件名，返回 ASCII 兜底名与百分号编码名。
// uuid 前缀避免同名文件在下载目录里互相覆盖。
func attachmentName(sheetName string) (ascii, encoded string) {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	if sheetName == "" {
		sheetName = "sheet1"
	}
	name := id + "_" + sheetName + ".xlsx"
	// 兜底名去掉所有非 ASCII，中文表名会只剩 uuid 部分，仍是个合法文件名。
	return id + ".xlsx", percentEncode(name)
}

// percentEncode 百分号编码文件名，只放过 RFC 3986 的 unreserved 字符。
//
// 不用 url.QueryEscape（空格编成 +，而文件名里的 + 是字面加号，浏览器不会还原成空格），
// 也不用 url.PathEscape（它放过 = 和 @，二者都不是 RFC 5987 允许的 attr-char，会把 filename* 的值截断）。
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9',
			ch == '-', ch == '.', ch == '_', ch == '~':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
