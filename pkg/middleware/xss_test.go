package middleware

import (
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// hutoolRegex 是 hutool RE_HTML_MARK 的原值（三段或）。
//
// 只在测试里出现：生产代码用的是化简后的单段版本，用它来证明化简没改行为。
var hutoolRegex = regexp.MustCompile(`(<[^<]*?>)|(<[\s]*?/[^<]*?>)|(<[^<]*?/[\s]*?>)`)

// newXSSEngine 构造 RepeatableBody + XSS + 回显 handler 的引擎。
//
// handler 回显它**实际收到**的参数 —— 这个中间件的全部意义就是让 handler
// 拿到清洗后的值，所以断言必须落在 handler 侧，而不是中间件内部状态。
func newXSSEngine(cfg XSSConfig) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(RepeatableBody())
	r.Use(XSSWithConfig(cfg))

	echo := func(c *gin.Context) {
		out := gin.H{
			"query": c.Query("q"),
			"form":  c.PostForm("f"),
		}
		// JSON 请求额外回显绑定结果，验证结构没被破坏。
		if isJSONRequest(c) {
			var body map[string]any
			if err := c.ShouldBindJSON(&body); err != nil {
				out["bindErr"] = err.Error()
			} else {
				out["body"] = body
			}
		}
		c.JSON(http.StatusOK, out)
	}
	for _, m := range []string{"GET", "POST", "PUT", "DELETE"} {
		r.Handle(m, "/test", echo)
		r.Handle(m, "/system/notice", echo)
	}
	return r
}

// doXSS 发一次请求并解析回显结果。
func doXSS(t *testing.T, r *gin.Engine, method, target, contentType, body string) map[string]any {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析回显失败: %v, body=%s", err, w.Body.String())
	}
	return out
}

// 核心用例：JSON 体里的标签被剔掉，且 handler 仍能正常绑定。
func TestXSSCleansJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	out := doXSS(t, r, http.MethodPost, "/test", "application/json",
		`{"name":"<script>alert(1)</script>hello"}`)

	if err, ok := out["bindErr"]; ok {
		t.Fatalf("清洗后 handler 绑定失败: %v", err)
	}
	body, _ := out["body"].(map[string]any)
	if got := body["name"]; got != "alert(1)hello" {
		t.Errorf("name = %q, 期望 %q", got, "alert(1)hello")
	}
}

// 关键回归：Java 侧对整串做正则会破坏 JSON 结构，Go 侧逐值清洗必须不会。
//
// {"a":"1<2","b":"3>4"} 在 Java 侧会变成 {"a":"14"} —— b 字段凭空消失。
// 这条用例锁住「结构完整」这个相对原项目的有意偏差，别为了对齐 Java 改回去。
func TestXSSDoesNotCorruptJSONStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	out := doXSS(t, r, http.MethodPost, "/test", "application/json",
		`{"a":"1<2","b":"3>4"}`)

	if err, ok := out["bindErr"]; ok {
		t.Fatalf("绑定失败，JSON 结构可能被破坏: %v", err)
	}
	body, _ := out["body"].(map[string]any)
	if body["a"] != "1<2" {
		t.Errorf("a = %q, 期望原样保留 %q", body["a"], "1<2")
	}
	if body["b"] != "3>4" {
		t.Errorf("b = %q, 期望原样保留 %q（Java 侧这里会整个丢字段）", body["b"], "3>4")
	}

	// 顺带证明：对整串清洗确实会造成上述损坏。
	if got := hutoolRegex.ReplaceAllString(`{"a":"1<2","b":"3>4"}`, ""); got != `{"a":"14"}` {
		t.Errorf("hutool 整串清洗结果 = %q，与预期的损坏形态不符（用例前提变了）", got)
	}
}

// 嵌套结构里的标签也要清掉，且数字不能因浮点解析丢精度。
func TestXSSCleansNestedAndPreservesNumbers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 直接读 handler 收到的原始 body：经回显 JSON 往返会被解成 float64，
	// 那样就测不出中间件有没有丢精度了。
	var got string
	r := gin.New()
	r.Use(RepeatableBody(), XSS())
	r.POST("/nested", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		c.Status(http.StatusOK)
	})

	// 1761100000000000001 是雪花 id 的量级，float64 只有 53 位有效位，
	// 会把尾数抹成 ...0000 —— 那就是把 handler 要用的主键改成一个不存在的值。
	req := httptest.NewRequest(http.MethodPost, "/nested", strings.NewReader(
		`{"users":[{"bio":"<b>hi</b>","id":1761100000000000001}],"ok":true,"n":null}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(got, `1761100000000000001`) {
		t.Errorf("雪花 id 被改写，清洗后 body = %s", got)
	}
	if !strings.Contains(got, `"bio":"hi"`) {
		t.Errorf("嵌套 bio 未被清洗，清洗后 body = %s", got)
	}
	if !strings.Contains(got, `"ok":true`) || !strings.Contains(got, `"n":null`) {
		t.Errorf("bool/null 字段被改写，清洗后 body = %s", got)
	}
}

// 顶层就是字符串的 JSON（合法）也要清洗 —— 这是 cleanJSONNode 必须返回值的原因。
func TestXSSCleansTopLevelJSONString(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	r := gin.New()
	r.Use(RepeatableBody(), XSS())
	r.POST("/s", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/s", strings.NewReader(`"<b>x</b>"`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got != `"x"` {
		t.Errorf("顶层字符串 body = %s, 期望 %s", got, `"x"`)
	}
}

// 查询串要被清洗，含 percent 编码的载荷同样要拦下。
func TestXSSCleansQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	cases := []struct{ name, target, want string }{
		{"裸标签", "/test?q=<script>x</script>", "x"},
		{"percent 编码", "/test?q=%3Cscript%3Ex%3C%2Fscript%3E", "x"},
		{"无标签保持原样", "/test?q=1+%3C+2", "1 < 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 用 POST：GET 按原项目行为整体跳过清洗。
			out := doXSS(t, r, http.MethodPost, tc.target, "", "")
			if out["query"] != tc.want {
				t.Errorf("query = %q, 期望 %q", out["query"], tc.want)
			}
		})
	}
}

// 表单字段要被清洗。
func TestXSSCleansForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	out := doXSS(t, r, http.MethodPost, "/test",
		"application/x-www-form-urlencoded", "f=%3Cb%3Ebold%3C%2Fb%3E")

	if out["form"] != "bold" {
		t.Errorf("form = %q, 期望 %q", out["form"], "bold")
	}
}

// GET / DELETE 整体跳过，对齐 XssFilter.handleExcludeURL。
//
// 这同时是那个已知缺口的**行为锁**：它记录「GET 查询参数不被清洗」是
// 有意对齐原项目，而不是漏了 —— 真正的防线在输出侧。
func TestXSSSkipsGetAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	for _, m := range []string{http.MethodGet, http.MethodDelete} {
		t.Run(m, func(t *testing.T) {
			out := doXSS(t, r, m, "/test?q=%3Cb%3Ex%3C%2Fb%3E", "", "")
			if out["query"] != "<b>x</b>" {
				t.Errorf("query = %q, 期望原样未清洗 %q", out["query"], "<b>x</b>")
			}
		})
	}
}

// excludeUrls 命中的路径整体跳过，富文本原样送达。
func TestXSSSkipsExcludedURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newXSSEngine(DefaultXSSConfig())

	raw := `{"content":"<p>公告正文</p>"}`
	out := doXSS(t, r, http.MethodPost, "/system/notice", "application/json", raw)
	body, _ := out["body"].(map[string]any)
	if body["content"] != "<p>公告正文</p>" {
		t.Errorf("content = %q, 期望富文本原样保留", body["content"])
	}

	// 带查询串时仍要命中排除规则（Java 用 getServletPath，不含查询串）。
	out = doXSS(t, r, http.MethodPost, "/system/notice?q=%3Cb%3Ex%3C%2Fb%3E",
		"application/json", raw)
	if out["query"] != "<b>x</b>" {
		t.Errorf("带查询串时未命中排除规则: query = %q", out["query"])
	}
}

// 清洗后 ContentLength 与 body 必须一致，且 ShouldBindBodyWith 也能读到清洗后的值。
func TestXSSUpdatesContentLengthAndBodyCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var length int64
	var viaCache map[string]any
	r := gin.New()
	r.Use(RepeatableBody(), XSS())
	r.POST("/c", func(c *gin.Context) {
		length = c.Request.ContentLength
		_ = c.ShouldBindBodyWith(&viaCache, binding.JSON)
		b, _ := io.ReadAll(c.Request.Body)
		if int64(len(b)) != length {
			t.Errorf("ContentLength=%d 与实际 body 长度=%d 不一致", length, len(b))
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/c",
		strings.NewReader(`{"a":"<b>x</b>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if viaCache["a"] != "x" {
		t.Errorf("ShouldBindBodyWith 读到 %q, 期望清洗后的 %q", viaCache["a"], "x")
	}
}

// 非法 JSON 原样放行：交给 handler 的绑定去拒，中间件不掺和。
func TestXSSLeavesInvalidJSONUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	r := gin.New()
	r.Use(RepeatableBody(), XSS())
	r.POST("/i", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		c.Status(http.StatusOK)
	})

	raw := `{"a":"<b>x</b>` // 少个收尾括号
	req := httptest.NewRequest(http.MethodPost, "/i", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got != raw {
		t.Errorf("非法 JSON 被改动 = %q, 期望原样 %q", got, raw)
	}
}

// 无标签时不重新序列化 —— 字段顺序与数字格式保持原样。
func TestXSSSkipsRewriteWhenNothingChanged(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	r := gin.New()
	r.Use(RepeatableBody(), XSS())
	r.POST("/n", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		c.Status(http.StatusOK)
	})

	// 含 `<` 但不构成标签，会走解析但不该触发重写。
	raw := `{"z":"1 < 2","a":1.50}`
	req := httptest.NewRequest(http.MethodPost, "/n", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got != raw {
		t.Errorf("无改动时 body 被重写 = %q, 期望原样 %q", got, raw)
	}
}

// 化简后的单段正则与 hutool 的三段或行为一致（固定用例 + 随机交叉验证）。
func TestCleanHTMLTagMatchesHutoolRegex(t *testing.T) {
	fixed := []string{
		"<script>alert(1)</script>", "</p>", "<br/>", "< /p>", "<br/ >",
		"<a href='x'>y</a>", "<", ">", "<a<b>", "<b>粗体</b>", "1<2>3",
		"no tags here", "", "<<>>", "<img src=x onerror=alert(1)>",
		"<div\n class='a'>x</div>", "a<b", "<>", "<b>x",
	}
	for _, s := range fixed {
		if got, want := cleanHTMLTag(s), hutoolRegex.ReplaceAllString(s, ""); got != want {
			t.Errorf("cleanHTMLTag(%q) = %q, hutool = %q", s, got, want)
		}
	}

	// 随机串：只用容易触发正则边界的字符表，比纯随机字节更可能撞出差异。
	alphabet := []rune(`<>/ abc"'=`)
	for i := 0; i < 3000; i++ {
		n := rand.IntN(12)
		var sb strings.Builder
		for j := 0; j < n; j++ {
			sb.WriteRune(alphabet[rand.IntN(len(alphabet))])
		}
		s := sb.String()
		if got, want := cleanHTMLTag(s), hutoolRegex.ReplaceAllString(s, ""); got != want {
			t.Fatalf("随机用例不一致: %q -> 本实现 %q, hutool %q", s, got, want)
		}
	}
}

// cleanValues 就地改写并正确报告是否有改动。
func TestCleanValues(t *testing.T) {
	v := url.Values{"a": {"<b>x</b>", "plain"}, "b": {" <i>y</i> "}}
	if !cleanValues(v) {
		t.Error("cleanValues 应报告有改动")
	}
	if v["a"][0] != "x" || v["a"][1] != "plain" || v["b"][0] != "y" {
		t.Errorf("清洗结果不符: %v", v)
	}
	if cleanValues(url.Values{"a": {"plain"}}) {
		t.Error("无改动时应返回 false")
	}
}

// 全链路顺序验证：Recover → TraceID → RepeatableBody → AccessLog → XSS
// 三处入参（查询串 / 表单 / JSON 体）都要被清洗。
//
// 这条用例的重点是 AccessLog 排在 XSS 之前 —— 它会先调一次 ParseForm，
// 必须确认那不会让 XSS 的表单清洗失效（Form 已解析时仍要就地改写）。
func TestXSSInFullChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var q, f, body string
	r := gin.New()
	r.Use(Recover(), TraceID(), RepeatableBody(), AccessLog(), XSS())
	r.POST("/full", func(c *gin.Context) {
		q, f = c.Query("q"), c.PostForm("f")
		b, _ := io.ReadAll(c.Request.Body)
		body = string(b)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/full?q=%3Cb%3EQ%3C%2Fb%3E",
		strings.NewReader("f=%3Ci%3EF%3C%2Fi%3E"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if q != "Q" || f != "F" {
		t.Errorf("全链路下未清洗: query=%q form=%q, 期望 Q / F", q, f)
	}

	req = httptest.NewRequest(http.MethodPost, "/full",
		strings.NewReader(`{"a":"<b>x</b>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if body != `{"a":"x"}` {
		t.Errorf("全链路下 JSON 未清洗 = %s, 期望 {\"a\":\"x\"}", body)
	}
}

// 锁住注册顺序约束：XSS **之前**若有中间件读过 gin 参数，
// gin 会把 URL.Query() 的结果缓存进 queryCache，之后再改 RawQuery 不生效。
//
// 这条用例记录的是一个**真实存在的失效条件**，不是期望行为 ——
// 它的意义在于：将来若有人把读参数的中间件（如提前解析 clientid 的鉴权）
// 挪到 XSS 之前，这里会以「清洗突然失效」的形式暴露出来，
// 而不是变成一个线上才发现的静默漏洞。
func TestXSSIneffectiveIfQueryReadEarlier(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	r := gin.New()
	// 故意在 XSS 之前读一次 —— 模拟被错误前置的中间件。
	r.Use(func(c *gin.Context) { _ = c.Query("q"); c.Next() })
	r.Use(XSS())
	r.POST("/q", func(c *gin.Context) {
		got = c.Query("q")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/q?q=%3Cb%3Ex%3C%2Fb%3E", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	if got != "<b>x</b>" {
		t.Errorf("got = %q。若已被清洗，说明 gin 的 queryCache 行为变了，"+
			"xss.go 里「XSS 必须在所有读参数的中间件之前」那条注释需要重新核实", got)
	}
}
