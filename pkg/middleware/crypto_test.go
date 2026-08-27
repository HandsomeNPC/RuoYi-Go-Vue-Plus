package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// testKeyPair 一对现生成的测试密钥。
type testKeyPair struct {
	priv       *rsa.PrivateKey
	privBase64 string
	pubBase64  string
}

func newTestKeyPair(t *testing.T) testKeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	return testKeyPair{
		priv:       key,
		privBase64: base64.StdEncoding.EncodeToString(privDER),
		pubBase64:  base64.StdEncoding.EncodeToString(pubDER),
	}
}

// clientEncrypt 模拟前端的加密过程，返回密钥头与密文体。
func clientEncrypt(t *testing.T, plaintext, aesPassword string, pub *rsa.PublicKey) (header, body string) {
	t.Helper()

	// 里层 base64：协议规定头里放的是 base64 编码后的 AES 密钥。
	encoded := base64.StdEncoding.EncodeToString([]byte(aesPassword))
	encryptedKey, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(encoded))
	if err != nil {
		t.Fatalf("模拟前端 RSA 加密失败: %v", err)
	}

	cipherBody, err := encrypt.EncryptByAES(plaintext, aesPassword)
	if err != nil {
		t.Fatalf("模拟前端 AES 加密失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(encryptedKey), cipherBody
}

// rsaEncryptHeader 只构造密钥头，不加密请求体。
func rsaEncryptHeader(t *testing.T, aesPassword string, pub *rsa.PublicKey) string {
	t.Helper()
	encoded := base64.StdEncoding.EncodeToString([]byte(aesPassword))
	out, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(encoded))
	if err != nil {
		t.Fatalf("模拟前端 RSA 加密失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(out)
}

// newCryptoEngine 构造完整链路：APIEncrypt → RepeatableBody → AccessLog → XSS。
func newCryptoEngine(cfg config.APIEncryptConfig) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(TraceID())
	r.Use(APIEncryptWithConfig(cfg))
	r.Use(RepeatableBody())
	r.Use(AccessLog())
	r.Use(XSS())

	echo := func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusOK, gin.H{"bindErr": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"body":        body,
			"contentType": c.ContentType(),
			"cached":      string(BodyBytes(c)),
		})
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodGet} {
		r.Handle(m, "/auth/login", echo)
		r.Handle(m, "/open/plain", echo)
	}
	return r
}

// enabledConfig 一份启用了解密的配置。
func enabledConfig(kp testKeyPair) config.APIEncryptConfig {
	return config.APIEncryptConfig{
		Enabled:     true,
		HeaderFlag:  config.DefaultAPIEncryptHeader,
		PrivateKey:  kp.privBase64,
		PublicKey:   kp.pubBase64,
		RequestURLs: []string{"/auth/login"},
		MaxBodySize: 10 << 20,
	}
}

// TestAPIEncryptDecryptsRequest 带密钥头的加密请求应被解密，handler 绑到明文。
func TestAPIEncryptDecryptsRequest(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	const plaintext = `{"username":"admin","password":"admin123"}`
	header, body := clientEncrypt(t, plaintext, "1234567890123456", &kp.priv.PublicKey)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	// 前端发的是 base64 纯文本，不是 json —— 这正是中间件必须改写 Content-Type 的原因。
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
	}
	if e, ok := got["bindErr"]; ok {
		t.Fatalf("handler 绑定失败: %v", e)
	}

	bodyMap, _ := got["body"].(map[string]any)
	if bodyMap["username"] != "admin" || bodyMap["password"] != "admin123" {
		t.Errorf("handler 绑到的明文不对: %v", bodyMap)
	}
	if ct, _ := got["contentType"].(string); ct != ContentTypeJSON {
		t.Errorf("Content-Type = %q, want %q", ct, ContentTypeJSON)
	}
	if cached, _ := got["cached"].(string); cached != plaintext {
		t.Errorf("RepeatableBody 缓存 = %q, want %q", cached, plaintext)
	}
}

// TestAPIEncryptAllAESKeySizes 三种密钥长度都要能解。
func TestAPIEncryptAllAESKeySizes(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	for _, key := range []string{
		"1234567890123456",
		"123456789012345678901234",
		"12345678901234567890123456789012",
	} {
		t.Run(key[:4]+"...", func(t *testing.T) {
			const plaintext = `{"username":"admin"}`
			header, body := clientEncrypt(t, plaintext, key, &kp.priv.PublicKey)

			req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set(config.DefaultAPIEncryptHeader, header)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if !strings.Contains(w.Body.String(), "admin") {
				t.Errorf("解密失败: %s", w.Body.String())
			}
		})
	}
}

// TestAPIEncryptHandlesPUT PUT 也要解密。
func TestAPIEncryptHandlesPUT(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	header, body := clientEncrypt(t, `{"password":"new"}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPut, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "new") {
		t.Errorf("PUT 请求未被解密: %s", w.Body.String())
	}
}

// TestAPIEncryptRejectsPlaintextOnRequiredPath 命中强制加密清单但没带密钥头 → 拒绝。
func TestAPIEncryptRejectsPlaintextOnRequiredPath(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"username":"admin","password":"admin123"}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want 200（业务码走响应体）", w.Code)
	}
	var body response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
	}
	if body.Code != response.CodeForbidden {
		t.Errorf("业务码 = %d, want %d", body.Code, response.CodeForbidden)
	}
	if body.Msg != msgEncryptRequired {
		t.Errorf("文案 = %q, want %q", body.Msg, msgEncryptRequired)
	}
}

// TestAPIEncryptAllowsPlaintextOnOtherPaths 不在清单上的路径发明文照常放行。
func TestAPIEncryptAllowsPlaintextOnOtherPaths(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	req := httptest.NewRequest(http.MethodPost, "/open/plain",
		strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `"a"`) {
		t.Errorf("未加密路径的明文请求应放行: %s", w.Body.String())
	}
}

// TestAPIEncryptDecryptsAnyPathWithHeader 带了密钥头就解密，与路径清单无关。
func TestAPIEncryptDecryptsAnyPathWithHeader(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	header, body := clientEncrypt(t, `{"a":"ok"}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/open/plain", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("带密钥头的请求应被解密（不论路径）: %s", w.Body.String())
	}
}

// TestAPIEncryptSkipsGET GET 不解密。
func TestAPIEncryptSkipsGET(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	req.Header.Set(config.DefaultAPIEncryptHeader, "whatever")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 没有 body 会报 EOF，关键是没出现解密失败。
	if strings.Contains(w.Body.String(), msgDecryptFailed) {
		t.Errorf("GET 不该走解密: %s", w.Body.String())
	}
}

// TestAPIEncryptFailuresAreIndistinguishable 各种解密失败都必须回同一句文案，不泄漏失败阶段。
func TestAPIEncryptFailuresAreIndistinguishable(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	_, goodBody := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	goodHeader, _ := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)

	// 用另一把密钥加密的头。
	other := newTestKeyPair(t)
	wrongKeyHeader, _ := clientEncrypt(t, `{"a":1}`, "1234567890123456", &other.priv.PublicKey)

	// 头里装的 AES 密钥长度非法（15 字节）。
	badLenHeader := rsaEncryptHeader(t, "123456789012345", &kp.priv.PublicKey)

	tests := map[string]struct{ header, body string }{
		"头非 base64":      {"!!!not-base64!!!", goodBody},
		"头是垃圾密文":         {base64.StdEncoding.EncodeToString(make([]byte, 128)), goodBody},
		"头用了别的公钥":        {wrongKeyHeader, goodBody},
		"头长度非密钥长度整数倍":    {base64.StdEncoding.EncodeToString(make([]byte, 100)), goodBody},
		"AES 密钥长度非法":     {badLenHeader, goodBody},
		"体非 base64":      {goodHeader, "!!!not-base64!!!"},
		"体长度非分组整数倍":      {goodHeader, base64.StdEncoding.EncodeToString(make([]byte, 17))},
		"体是垃圾密文(填充必然非法)": {goodHeader, base64.StdEncoding.EncodeToString(make([]byte, 32))},
		"体为空":            {goodHeader, ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set(config.DefaultAPIEncryptHeader, tc.header)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var body response.R[any]
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
			}
			// 全部折叠成同一句。
			if body.Msg != msgDecryptFailed {
				t.Errorf("文案 = %q, want %q（所有解密失败必须无法区分）",
					body.Msg, msgDecryptFailed)
			}
			if body.Code != response.CodeFail {
				t.Errorf("业务码 = %d, want %d", body.Code, response.CodeFail)
			}
		})
	}
}

// TestAPIEncryptAbortsOnFailure 解密失败必须中止请求，绝不能把密文当明文交给 handler。
func TestAPIEncryptAbortsOnFailure(t *testing.T) {
	kp := newTestKeyPair(t)

	reached := false
	r := gin.New()
	r.Use(Recover())
	r.Use(APIEncryptWithConfig(enabledConfig(kp)))
	r.POST("/auth/login", func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{})
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("garbage"))
	req.Header.Set(config.DefaultAPIEncryptHeader, "!!!")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if reached {
		t.Error("解密失败后 handler 仍被执行 —— 密文会被当明文处理")
	}
}

// TestAPIEncryptDisabled 关闭时应完全放行。
func TestAPIEncryptDisabled(t *testing.T) {
	r := newCryptoEngine(config.APIEncryptConfig{Enabled: false})

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"username":"admin"}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("关闭时应原样放行: %s", w.Body.String())
	}
}

// TestAPIEncryptRejectsOversizedBody 超过上限的密文要被拒。
func TestAPIEncryptRejectsOversizedBody(t *testing.T) {
	kp := newTestKeyPair(t)
	cfg := enabledConfig(kp)
	cfg.MaxBodySize = 128

	r := newCryptoEngine(cfg)
	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(strings.Repeat("A", 512)))
	req.Header.Set(config.DefaultAPIEncryptHeader, "x")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if !strings.Contains(body.Msg, "超出大小限制") {
		t.Errorf("文案 = %q, want 包含「超出大小限制」", body.Msg)
	}
}

// TestAPIEncryptDecryptedBodyGetsSanitizedInLog 解密后的明文必须能被 AccessLog 脱敏。
func TestAPIEncryptDecryptedBodyGetsSanitizedInLog(t *testing.T) {
	const plaintext = `{"username":"admin","password":"admin123"}`

	got := jsonParamLog([]byte(plaintext), 4000)
	if strings.Contains(got, "admin123") {
		t.Errorf("解密后的明文密码进了日志: %s", got)
	}
	if !strings.Contains(got, "admin") {
		t.Errorf("非敏感字段应保留: %s", got)
	}
}

// TestAPIEncryptBlankHeaderTreatedAsPlaintext 空密钥头视为未加密，不该当解密失败。
func TestAPIEncryptBlankHeaderTreatedAsPlaintext(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"a":1}`))
	req.Header.Set(config.DefaultAPIEncryptHeader, "   ")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Msg != msgEncryptRequired {
		t.Errorf("文案 = %q, want %q（空白头应视为未加密而非解密失败）",
			body.Msg, msgEncryptRequired)
	}
}

// TestAPIEncryptPanicsOnBadKey 启用但私钥非法时应 panic。
func TestAPIEncryptPanicsOnBadKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("私钥非法时应 panic（启动期编排错误）")
		}
	}()
	APIEncryptWithConfig(config.APIEncryptConfig{
		Enabled:    true,
		PrivateKey: "not-a-key",
	})
}

// TestAPIEncryptDeliversHandlerErrors handler 只 c.Error 不写 body 时，错误响应必须仍能送达客户端。
func TestAPIEncryptDeliversHandlerErrors(t *testing.T) {
	kp := newTestKeyPair(t)

	r := gin.New()
	r.Use(Recover())
	r.Use(APIEncryptWithConfig(responseConfig(kp)))
	r.POST("/auth/login", func(c *gin.Context) {
		// 只登记错误、不写响应体。
		_ = c.Error(errs.New(0, "用户名或密码错误", ""))
	})

	header, body := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.Len() == 0 {
		t.Fatal("错误响应体为空 —— c.Writer 没还原，Recover 的响应被丢进了缓冲区")
	}
	var got response.R[any]
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
	}
	if got.Msg != "用户名或密码错误" {
		t.Errorf("文案 = %q, want %q（handler 登记的业务错误应原样送达）",
			got.Msg, "用户名或密码错误")
	}

	// 这份响应没加密，密钥头必须撤掉。
	if v := w.Header().Get(config.DefaultAPIEncryptHeader); v != "" {
		t.Errorf("响应未加密却仍带密钥头 %q —— 前端会拿它去解一段明文 JSON", v)
	}
}

// ---------- 响应加密 ----------
//
// 以下几条覆盖 @ApiEncrypt(response = true)。**原项目 4 处 @ApiEncrypt 全是
// 默认的 response = false**，即这条链路在原项目里从未启用过，
// 所以没有可对照的线上行为，仅按 EncryptResponseBodyWrapper 的代码复刻。

// responseConfig 一份启用了响应加密的配置。
//
// 响应加密用的公钥在真实部署里是**前端的**公钥（服务端加密、前端用配套
// 私钥解密）。测试里用同一对，好让我们能解开来验证。
func responseConfig(kp testKeyPair) config.APIEncryptConfig {
	cfg := enabledConfig(kp)
	cfg.ResponseURLs = []string{"/auth/login"}
	return cfg
}

// TestAPIEncryptEncryptsResponse 响应加密：body 应变成密文，密钥头应能用私钥解开。
func TestAPIEncryptEncryptsResponse(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(responseConfig(kp))

	header, body := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 响应体不再是明文 JSON。
	if strings.Contains(w.Body.String(), `"body"`) {
		t.Fatalf("响应体未被加密: %s", w.Body.String())
	}

	// 走完整的解密流程还原它，即模拟前端的解密侧。
	respKeyHeader := w.Header().Get(config.DefaultAPIEncryptHeader)
	if respKeyHeader == "" {
		t.Fatal("响应缺少密钥头，前端无从解密")
	}
	encodedKey, err := encrypt.DecryptByRSA(respKeyHeader, kp.priv)
	if err != nil {
		t.Fatalf("RSA 解密响应密钥失败: %v", err)
	}
	aesPassword, err := encrypt.DecryptByBase64(encodedKey)
	if err != nil {
		t.Fatalf("响应密钥 base64 解码失败（双层套嵌可能没对齐）: %v", err)
	}
	plain, err := encrypt.DecryptByAES(w.Body.String(), aesPassword)
	if err != nil {
		t.Fatalf("AES 解密响应体失败: %v", err)
	}
	if !strings.Contains(plain, `"body"`) {
		t.Errorf("解密后的响应体不对: %s", plain)
	}
}

// TestAPIEncryptResponseKeyIsPerRequest 响应密钥必须每次请求都不同。
func TestAPIEncryptResponseKeyIsPerRequest(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(responseConfig(kp))

	send := func() (headerValue, body string) {
		h, b := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(b))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set(config.DefaultAPIEncryptHeader, h)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Header().Get(config.DefaultAPIEncryptHeader), w.Body.String()
	}

	k1, b1 := send()
	k2, b2 := send()
	if k1 == k2 {
		t.Error("两次请求的响应密钥相同 —— 一次性密钥失效，密文会成为可比对的指纹")
	}
	// 密钥不同 ⇒ 同一明文的密文也应不同。
	if b1 == b2 {
		t.Error("两次响应的密文相同，密钥可能没真正参与加密")
	}
}

// TestAPIEncryptLeavesOtherResponsesPlain 未命中 responseUrls 的路径响应不该被加密。
func TestAPIEncryptLeavesOtherResponsesPlain(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(responseConfig(kp))

	req := httptest.NewRequest(http.MethodPost, "/open/plain", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), `"body"`) {
		t.Errorf("未命中 responseUrls 的响应不该被加密: %s", w.Body.String())
	}
	if w.Header().Get(config.DefaultAPIEncryptHeader) != "" {
		t.Error("未加密的响应不该带密钥头")
	}
}

// TestAPIEncryptExposesKeyHeaderWithoutClobbering 密钥头必须出现在 Expose-Headers 里，且不能顶掉 X-Request-Id。
func TestAPIEncryptExposesKeyHeaderWithoutClobbering(t *testing.T) {
	kp := newTestKeyPair(t)

	r := gin.New()
	r.Use(Recover())
	r.Use(CORS())
	r.Use(TraceID())
	r.Use(APIEncryptWithConfig(responseConfig(kp)))
	r.POST("/auth/login", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": 1}) })

	header, body := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)
	req.Header.Set("Origin", "https://example.com")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	exposed := strings.Join(w.Header().Values("Access-Control-Expose-Headers"), ", ")
	if !strings.Contains(exposed, config.DefaultAPIEncryptHeader) {
		t.Errorf("Expose-Headers 缺少密钥头，跨域下前端读不到: %q", exposed)
	}
	if !strings.Contains(exposed, TraceIDHeader) {
		t.Errorf("Expose-Headers 里的 %s 被顶掉了，前端将拿不到 traceId: %q",
			TraceIDHeader, exposed)
	}
}

// TestAPIEncryptRewritesContentLength Content-Length 必须按密文长度重写。
func TestAPIEncryptRewritesContentLength(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(responseConfig(kp))

	header, body := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := w.Header().Get("Content-Length")
	if got == "" {
		t.Fatal("响应缺少 Content-Length")
	}
	if want := strconv.Itoa(w.Body.Len()); got != want {
		t.Errorf("Content-Length = %s, want %s（实际密文长度）—— 不一致会让客户端截断密文",
			got, want)
	}
}
