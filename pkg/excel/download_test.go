package excel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/response"
)

func TestPercentEncode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"纯 ASCII 不变", "abc.xlsx", "abc.xlsx"},
		{"中文全编码", "客户端管理.xlsx", "%E5%AE%A2%E6%88%B7%E7%AB%AF%E7%AE%A1%E7%90%86.xlsx"},
		{"空格编成 %20 而非 +", "a b.xlsx", "a%20b.xlsx"},
		{"分号逗号等参数字符全编码", "a;b,c.xlsx", "a%3Bb%2Cc.xlsx"},
		{"等号和 @ 也编码", "a&b=c@d.xlsx", "a%26b%3Dc%40d.xlsx"},
		{"加号编码", "a+b.xlsx", "a%2Bb.xlsx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentEncode(tt.in)
			if got != tt.want {
				t.Errorf("percentEncode(%q) = %q, 期望 %q", tt.in, got, tt.want)
			}
			// 往返还原，保证没有编出不可逆的字节。
			if back, err := url.QueryUnescape(got); err != nil || back != tt.in {
				t.Errorf("percentEncode 往返 %q → %q (err=%v)", tt.in, back, err)
			}
		})
	}
}

func TestAttachmentNameShapes(t *testing.T) {
	ascii, encoded := attachmentName("客户端管理")

	// 兜底名 = 32 位 uuid 前缀 + .xlsx（中文字段被去掉）。
	if len(ascii) != 32+len(".xlsx") {
		t.Errorf("ascii 名长度 = %d, 期望 %d", len(ascii), 32+len(".xlsx"))
	}
	if !strings.HasSuffix(ascii, ".xlsx") {
		t.Errorf("ascii 名应以 .xlsx 结尾, 得到 %q", ascii)
	}
	// 兜底名必须纯 ASCII：header 里不能有非 ASCII 字节。
	for _, r := range ascii {
		if r > 127 {
			t.Errorf("ascii 名含非 ASCII 字符 %q", r)
		}
	}
	idPart := ascii[:32]
	if len(idPart) != 32 {
		t.Fatalf("uuid 部分长度 = %d", len(idPart))
	}
	for _, r := range idPart {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("uuid 部分含非十六进制字符 %q", r)
		}
	}
	if strings.Contains(ascii, "-") {
		t.Error("uuid 不应带横线")
	}

	// encoded 名含中文的百分号编码，仍以 .xlsx 结尾。
	if !strings.Contains(encoded, "%") {
		t.Error("encoded 名应对中文做百分号编码")
	}
	if !strings.HasSuffix(encoded, ".xlsx") {
		t.Errorf("encoded 名应以 .xlsx 结尾, 得到 %q", encoded)
	}

	// 两次调用生成不同文件名（uuid 前缀），避免下载目录互相覆盖。
	_, encoded2 := attachmentName("客户端管理")
	if encoded == encoded2 {
		t.Error("两次生成的文件名不该相同")
	}
}

func TestAttachmentHeader(t *testing.T) {
	got := attachment("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.xlsx", "%E4%B8%AD")
	want := `attachment; filename="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.xlsx"; filename*=UTF-8''%E4%B8%AD`
	if got != want {
		t.Errorf("attachment() = %q, 期望 %q", got, want)
	}
}

func TestExportHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/system/client/export", func(c *gin.Context) {
		_ = Export(c, []testRow{{ID: 123, Name: "x"}}, "客户端管理")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/client/export", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != contentType {
		t.Errorf("Content-Type = %q, 期望 %q", ct, contentType)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=") {
		t.Errorf("Content-Disposition = %q, 期望以 attachment; filename= 开头", cd)
	}
	if !strings.Contains(cd, "filename*=UTF-8''") {
		t.Errorf("Content-Disposition 缺 filename*, 得到 %q", cd)
	}

	// download-filename 是 Java 兼容的前端口径，解码后应得到带中文的文件名。
	df := w.Header().Get(HeaderDownloadFilename)
	if df == "" {
		t.Fatal("download-filename 头缺失")
	}
	decoded, err := url.QueryUnescape(df)
	if err != nil {
		t.Fatalf("解码 download-filename 失败: %v", err)
	}
	if !strings.Contains(decoded, "客户端管理.xlsx") {
		t.Errorf("解码后的文件名 = %q, 期望含 客户端管理.xlsx", decoded)
	}

	if cl := w.Header().Get("Content-Length"); cl == "" {
		t.Error("Content-Length 头缺失")
	}

	// xlsx 是 zip，开头是 PK 魔数。
	if body := w.Body.Bytes(); !strings.HasPrefix(string(body), "PK\x03\x04") {
		t.Errorf("响应体不是 zip 开头, 得到 %q", body[:4])
	}
}

// TestExportFailureWritesNothing 失败时先落缓存不落字节，错误走 JSON。
// 这是"先建缓冲再落笔"约束的可测试证据：若顺序反了，错误响应会以
// attachment 头 + xlsx 类型发出去，前端按文件存下来打开是一句 JSON。
func TestExportFailureWritesNothing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/system/client/export", func(c *gin.Context) {
		overLimit := make([]testRow, MaxRows+1)
		if err := Export(c, overLimit, "客户端管理"); err != nil {
			_ = c.Error(err)
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/client/export", nil)
	r.ServeHTTP(w, req)

	// 失败路径不写附件头，客户端看到的是普通 JSON 错误，而不是一个坏文件。
	if cd := w.Header().Get("Content-Disposition"); cd != "" {
		t.Errorf("失败时不该有 Content-Disposition, 得到 %q", cd)
	}
	if df := w.Header().Get(HeaderDownloadFilename); df != "" {
		t.Errorf("失败时不该有 download-filename, 得到 %q", df)
	}

	var resp response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("失败响应不是合法 JSON: %v", err)
	}
	if resp.Code != response.CodeFail {
		t.Errorf("失败响应 code = %d, 期望 %d", resp.Code, response.CodeFail)
	}
	if !strings.Contains(resp.Msg, "导出数据量过大") {
		t.Errorf("失败响应 msg = %q, 期望含 导出数据量过大", resp.Msg)
	}
}

func TestContentTypeConst(t *testing.T) {
	if contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("contentType = %q", contentType)
	}
}

// TestResponseCodeFailConstant 断言 pkg/response 的业务码，防止上游改掉后本包静默失效。
func TestResponseCodeFailConstant(t *testing.T) {
	if response.CodeFail != 500 {
		t.Errorf("response.CodeFail = %d, 期望 500", response.CodeFail)
	}
}

// 引用本包的 errs 构造器（超限错误走它），避免误删导入。
var _ = errs.New
