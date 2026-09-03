package sms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ruoyi-go-vue-plus/pkg/config"
)

// TestSignMatchesAliyunOfficialExample 用阿里云文档里的官方样例校验签名。
//
// 这是整个包里最值得钉死的一条：签名算法只要错一个字符，短信就一条都发不出去，
// 而错误只会表现为网关返回 SignatureDoesNotMatch，从代码上看不出哪步错了。
//
// 样例参数取自阿里云「RPC 签名机制」文档。两个期望值都保留：
// 文档正文给的是 GET 签名 OLeaidS1JvxuMvnyHOwuJ+uX5qY=，而 sms4j（及本实现）
// 走 POST，故本实现应得到 POST 那个值。两者只差待签串首段的动词——
// 一并断言能同时证明「规范化串正确」与「动词确实是 POST」。
func TestSignMatchesAliyunOfficialExample(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      "testid",
		"Action":           "DescribeRegions",
		"Format":           "XML",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf",
		"SignatureVersion": "1.0",
		"Timestamp":        "2016-02-23T12:46:24Z",
		"Version":          "2014-05-26",
	}
	// POST 动词下的签名（密钥 testsecret，签名时用 testsecret&）。
	const wantPOST = "MxbnVAM4w6sft9xjVpe/GCKueuk="
	// 文档正文列出的 GET 签名，用于反向确认动词没写错。
	const gotWithGET = "OLeaidS1JvxuMvnyHOwuJ+uX5qY="

	got := sign(params, "testsecret")
	if got == gotWithGET {
		t.Fatal("算出了 GET 的签名，说明待签串首段的动词写成了 GET；阿里云短信走 POST")
	}
	if got != wantPOST {
		t.Errorf("sign() = %q, want %q（签名算法与阿里云不一致，短信将全部发送失败）",
			got, wantPOST)
	}
}

// TestCanonicalizedQueryStringMatchesDoc 规范化查询串与文档逐字一致。
//
// 单独断言这一步：签名值对不上时，它能立刻区分是「规范化串拼错了」
// 还是「HMAC 那步错了」，否则只能盯着一串 Base64 猜。
func TestCanonicalizedQueryStringMatchesDoc(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      "testid",
		"Action":           "DescribeRegions",
		"Format":           "XML",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf",
		"SignatureVersion": "1.0",
		"Timestamp":        "2016-02-23T12:46:24Z",
		"Version":          "2014-05-26",
	}
	// 注意 Timestamp 里的冒号编成 %3A，而连字符与点保持字面量。
	const want = "AccessKeyId=testid&Action=DescribeRegions&Format=XML" +
		"&SignatureMethod=HMAC-SHA1" +
		"&SignatureNonce=3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf" +
		"&SignatureVersion=1.0&Timestamp=2016-02-23T12%3A46%3A24Z&Version=2014-05-26"

	if got := canonicalizedQuery(params); got != want {
		t.Errorf("规范化查询串不一致\ngot  = %s\nwant = %s", got, want)
	}
}

// TestSignKeyHasTrailingAmpersand 密钥末尾的 & 不可省。
// 漏掉它签名恒不通过，而这是最容易忽略的一处。
func TestSignKeyHasTrailingAmpersand(t *testing.T) {
	params := map[string]string{"A": "1"}
	if sign(params, "secret") == sign(params, "secret&") {
		t.Error("密钥末尾的 & 未参与计算，说明实现漏了它")
	}
}

// TestSignIsOrderIndependent 入参是 map，签名不能受遍历顺序影响。
// Go 的 map 遍历顺序随机，不排序会让同样的参数每次算出不同签名。
func TestSignIsOrderIndependent(t *testing.T) {
	params := map[string]string{
		"Zebra": "z", "Apple": "a", "Mango": "m", "Banana": "b", "Cherry": "c",
	}
	first := sign(params, "secret")
	for range 20 {
		if got := sign(params, "secret"); got != first {
			t.Fatalf("同样的参数算出不同签名(%q != %q)，说明未按键名排序", got, first)
		}
	}
}

// TestSpecialEncode 三条替换规则缺一不可，少一条签名就对不上。
func TestSpecialEncode(t *testing.T) {
	cases := map[string]string{
		// 空格要 %20 而非 QueryEscape 给的 +。
		"a b": "a%20b",
		// * 要编成 %2A，QueryEscape 会放过它。
		"a*b": "a%2Ab",
		// ~ 要保留字面量，QueryEscape 会编成 %7E。
		"a~b": "a~b",
		// 斜杠仍要编码（待签串里的 "/" 靠这条变成 %2F）。
		"/": "%2F",
		// 常规字符不受影响。
		"abc123": "abc123",
		// JSON 串是 TemplateParam 的实际形态，整体作为一个值参与签名。
		`{"code":"1234"}`: "%7B%22code%22%3A%221234%22%7D",
	}
	for in, want := range cases {
		if got := specialEncode(in); got != want {
			t.Errorf("specialEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSpecialEncodeNoPlusRemains 编码结果里不该残留 + 与 %7E。
func TestSpecialEncodeNoPlusRemains(t *testing.T) {
	got := specialEncode("hello world ~ * end")
	if strings.Contains(got, "+") {
		t.Errorf("%q 残留了 +，空格未转成 %%20", got)
	}
	if strings.Contains(got, "%7E") {
		t.Errorf("%q 残留了 %%7E，~ 未还原成字面量", got)
	}
	if strings.Contains(got, "*") {
		t.Errorf("%q 残留了 *，未转成 %%2A", got)
	}
}

// TestAliyunSendSuccess 走一个假网关，验证请求形态与成功判定。
func TestAliyunSendSuccess(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`{"Code":"OK","Message":"OK","RequestId":"x","BizId":"y"}`))
	}))
	defer srv.Close()

	s := newAliyunSender(config.SMSConfig{
		Enabled:         true,
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		SignName:        "测试签名",
		TemplateCode:    "SMS_001",
		Endpoint:        strings.TrimPrefix(srv.URL, "https://"),
		RegionID:        "cn-hangzhou",
	})
	// 假网关用自签证书，换掉客户端以跳过校验。
	s.client = srv.Client()

	if err := s.Send(context.Background(), "13800138000",
		map[string]string{"code": "1234"}); err != nil {
		t.Fatalf("Send 报错: %v", err)
	}

	// 签名与全部参数都要落在 form body 里（阿里云 POST 形态）。
	for _, want := range []string{
		"Signature", "AccessKeyId", "Action", "PhoneNumbers", "SignName",
		"TemplateCode", "TemplateParam", "SignatureNonce", "Timestamp",
	} {
		if len(gotForm[want]) == 0 {
			t.Errorf("请求体缺少参数 %s", want)
		}
	}
	if got := gotForm.Get("PhoneNumbers"); got != "13800138000" {
		t.Errorf("PhoneNumbers = %q", got)
	}
	// 模板参数须是 JSON 串，模板里的变量名是 code。
	if got := gotForm.Get("TemplateParam"); got != `{"code":"1234"}` {
		t.Errorf("TemplateParam = %q, want {\"code\":\"1234\"}", got)
	}
	if got := gotForm.Get("Action"); got != aliyunAction {
		t.Errorf("Action = %q, want %q", got, aliyunAction)
	}
}

// TestAliyunSendFailureSurfacesMessage 厂商报错时把 Message 透出去，
// 而不是笼统报「发送失败」——那条文案要给前端看，得是可诊断的。
func TestAliyunSendFailureSurfacesMessage(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Code":"isv.TEMPLATE_MISSING_PARAMETERS","Message":"模板缺少变量"}`))
	}))
	defer srv.Close()

	s := newAliyunSender(config.SMSConfig{
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	s.client = srv.Client()

	err := s.Send(context.Background(), "13800138000", map[string]string{"code": "1"})
	if err == nil {
		t.Fatal("Code != OK 时应报错")
	}
	if !strings.Contains(err.Error(), "模板缺少变量") {
		t.Errorf("错误未带上厂商文案: %v", err)
	}
}

// TestSendCodeDisabled 未开启短信时返回 ErrDisabled，不去连网关。
func TestSendCodeDisabled(t *testing.T) {
	restore := SetSender(nil)
	defer restore()

	if Enabled() {
		t.Error("Sender 为 nil 时 Enabled() 应为 false")
	}
	if err := SendCode(context.Background(), "13800138000", "1234"); err != ErrDisabled {
		t.Errorf("SendCode 返回 %v, want ErrDisabled", err)
	}
}

// TestSendCodePassesCodeParam 验证码以 code 为模板变量名下发（对齐 Java map.put("code", ...)）。
func TestSendCodePassesCodeParam(t *testing.T) {
	var got map[string]string
	restore := SetSender(senderFunc(func(_ context.Context, _ string, params map[string]string) error {
		got = params
		return nil
	}))
	defer restore()

	if err := SendCode(context.Background(), "13800138000", "9527"); err != nil {
		t.Fatalf("SendCode 报错: %v", err)
	}
	if got["code"] != "9527" {
		t.Errorf("模板参数 = %v, want map[code:9527]", got)
	}
}

// senderFunc 把函数适配成 Sender，供测试注入。
type senderFunc func(context.Context, string, map[string]string) error

func (f senderFunc) Send(ctx context.Context, phone string, params map[string]string) error {
	return f(ctx, phone, params)
}
