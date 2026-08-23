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

// testKeyPair 一对现生成的 1024 位测试密钥。
//
// 现生成而非复用 configs 里那对：那对的公私钥**不是一对**（各自对应前端的
// 另一半），没法在单进程内做完整的往返验证。原项目那对密钥本身抄没抄对，
// 由 pkg/encrypt 与 pkg/config 的用例负责。
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

// clientEncrypt 模拟**前端**的加密过程，返回密钥头与密文体。
//
// 这个函数是本文件的核心：它按协议独立实现一遍加密侧，
// 而不是调用被测代码的逆函数。用 middleware 自己的解密逻辑反推测试数据的话，
// 两边同时写错（比如都漏掉里层 base64）测试照样会绿。
//
//	encrypt-key 头 = base64(RSA公钥加密( base64( AES明文密钥 ) ))
//	请求体         = base64(AES-ECB加密( JSON 明文 ))
func clientEncrypt(t *testing.T, plaintext, aesPassword string, pub *rsa.PublicKey) (header, body string) {
	t.Helper()

	// 里层 base64：协议规定头里放的是**base64 编码后**的 AES 密钥。
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
//
// 给「AES 密钥本身非法」这类用例用：clientEncrypt 会先在自己的 AES 一步
// 失败，拿不到想要的畸形输入。
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
//
// 有意挂上后面这几环而不是只挂 APIEncrypt：本中间件的正确性**主要体现在
// 它与下游的配合**上（下游能不能拿到明文、能不能绑定参数、日志能不能脱敏），
// 单独测它只能验证「解密函数被调用了」这件没什么价值的事。
func newCryptoEngine(cfg config.APIEncrypt) *gin.Engine {
	r := gin.New()
	r.Use(Recover())
	r.Use(TraceID())
	r.Use(APIEncryptWithConfig(cfg))
	r.Use(RepeatableBody())
	r.Use(AccessLog())
	r.Use(XSS())

	// handler 回显它实际绑到的东西 —— 断言落在这里而非中间件内部状态。
	echo := func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusOK, gin.H{"bindErr": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"body":        body,
			"contentType": c.ContentType(),
			// 确认 RepeatableBody 确实缓存了明文（下游 @Log 等依赖它）。
			"cached": string(BodyBytes(c)),
		})
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodGet} {
		r.Handle(m, "/auth/login", echo)
		r.Handle(m, "/open/plain", echo)
	}
	return r
}

// enabledConfig 一份启用了解密的配置。
func enabledConfig(kp testKeyPair) config.APIEncrypt {
	return config.APIEncrypt{
		Enabled:     true,
		HeaderFlag:  config.DefaultAPIEncryptHeader,
		PrivateKey:  kp.privBase64,
		PublicKey:   kp.pubBase64,
		RequestURLs: []string{"/auth/login"},
		MaxBodySize: 10 << 20,
	}
}

// 正常路径：带密钥头的加密请求应被解密，handler 绑到明文。
//
// 这条同时验证四件必须同时成立的事，缺一个都会让下游出问题：
// 解密正确、Content-Type 被改成 json、RepeatableBody 缓存到明文、handler 能绑定。
func TestAPIEncryptDecryptsRequest(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	const plaintext = `{"username":"admin","password":"admin123"}`
	header, body := clientEncrypt(t, plaintext, "1234567890123456", &kp.priv.PublicKey)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	// 前端发的是 base64 纯文本，**不是** json —— 这正是中间件必须改写
	// Content-Type 的原因，否则下游全都不认。
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
		t.Errorf("Content-Type = %q, want %q —— 不改的话下游全不认",
			ct, ContentTypeJSON)
	}
	if cached, _ := got["cached"].(string); cached != plaintext {
		t.Errorf("RepeatableBody 缓存 = %q, want %q（应缓存明文而非密文）",
			cached, plaintext)
	}
}

// 三种密钥长度都要能解 —— 前端换密钥长度不该让服务端挂掉。
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

// PUT 也要解密，对齐 CryptoFilter 的 PUT || POST 判断。
//
// 四个 @ApiEncrypt 接口里有两个是 PUT（resetPwd / updatePwd），漏掉 PUT
// 会让改密码接口完全不可用。
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

// 命中强制加密清单但没带密钥头 → 拒绝。
//
// 对齐 CryptoFilter 里「有 @ApiEncrypt 注解却无加密标头就报 403」的分支。
// 这条拦的是「本该加密的接口收到了明文密码」。
func TestAPIEncryptRejectsPlaintextOnRequiredPath(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"username":"admin","password":"admin123"}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// HTTP 状态码恒 200，业务码在响应体里（见 README「两条硬约束」）。
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
		t.Errorf("文案 = %q, want %q（对齐原项目）", body.Msg, msgEncryptRequired)
	}
}

// 不在清单上的路径发明文照常放行 —— 绝大多数接口都是这样。
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

// 带了密钥头就解密，与路径清单无关 —— 对齐 Java（那边解密只看头）。
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

// GET 不解密，对齐 CryptoFilter 只处理 PUT/POST。
func TestAPIEncryptSkipsGET(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	// 即使带了密钥头，GET 也不该走解密（会被当普通请求处理）。
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	req.Header.Set(config.DefaultAPIEncryptHeader, "whatever")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 没有 body，handler 的 ShouldBindJSON 会报 EOF —— 关键是**没有**
	// 出现解密失败，说明确实跳过了解密。
	if strings.Contains(w.Body.String(), msgDecryptFailed) {
		t.Errorf("GET 不该走解密: %s", w.Body.String())
	}
}

// 各种解密失败都必须回**同一句**文案，且不泄漏失败阶段。
//
// 这是本文件最重要的一条安全断言：区分「填充错」与「解密错」等于提供一个
// padding oracle，攻击者能拿同一段密文反复试探、逐字节还原明文
// （Vaudenay 攻击）。真实原因只进日志。
func TestAPIEncryptFailuresAreIndistinguishable(t *testing.T) {
	kp := newTestKeyPair(t)
	r := newCryptoEngine(enabledConfig(kp))

	_, goodBody := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	goodHeader, _ := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)

	// 用另一把密钥加密的头 —— 我们的私钥解不开。
	other := newTestKeyPair(t)
	wrongKeyHeader, _ := clientEncrypt(t, `{"a":1}`, "1234567890123456", &other.priv.PublicKey)

	// 头里装的 AES 密钥长度非法（15 字节）。走不了 clientEncrypt ——
	// 那个 helper 自己就会先在 AES 一步失败，所以只加密头、不加密体。
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
			// 全部折叠成同一句，一个字都不能多 —— 多出来的信息就是 oracle。
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

// 解密失败必须**中止**请求，绝不能把密文当明文交给 handler。
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

// 关闭时应完全放行，不碰任何请求。
func TestAPIEncryptDisabled(t *testing.T) {
	// 关闭时连密钥都不该被要求（validate 也放行这种形态）。
	r := newCryptoEngine(config.APIEncrypt{Enabled: false})

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"username":"admin"}`))
	req.Header.Set("Content-Type", ContentTypeJSON)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("关闭时应原样放行: %s", w.Body.String())
	}
}

// 超过上限的密文要被拒，且不能读进内存。
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

// 解密后的明文必须能被 AccessLog 脱敏 —— 这正是「必须在 RepeatableBody
// 之前」那条顺序约束的**目的**。
//
// 顺序摆反的话日志里只有 base64 密文，脱敏形同虚设。但摆对了也意味着
// **明文密码会流进 jsonParamLog**，所以那条路径的脱敏必须真的生效。
// 本用例直接断言 jsonParamLog 对解密后的 body 会删掉 password。
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

// 空密钥头（只有空白）视为未加密，不该当解密失败。
//
// 对齐 Java 的 StringUtils.isNotBlank 判断。此时若路径在强制清单上，
// 应走「要求加密」那条分支而非「解密失败」。
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

// 启用但私钥非法时应 panic —— 那是启动期的编排错误。
//
// 与 config.Get() 未初始化时 panic 同源：这种错误留到运行期的表现是
// 「所有加密接口都失败而其余正常」，比启动失败难查得多。
func TestAPIEncryptPanicsOnBadKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("私钥非法时应 panic（启动期编排错误）")
		}
	}()
	APIEncryptWithConfig(config.APIEncrypt{
		Enabled:    true,
		PrivateKey: "not-a-key",
	})
}

// handler 只 c.Error 不写 body 时，错误响应必须仍能送达客户端。
//
// 这条是回归测试，锁住一个真实存在过的 bug：响应加密替换了 c.Writer 却没有
// 还原，而渲染错误响应的 Recover 在本中间件**之外**、发生在它返回之后 ——
// 于是那份错误响应写进了已经用完的缓冲区，客户端收到 200 空响应，
// 服务端日志里也只有一行业务异常，两边都看不出发生了什么。
//
// 顺带锁住配套的一点：这条路径上的响应**没有被加密**（Recover 在链外，
// 顺序上无法两全），所以必须撤掉密钥头 —— 否则前端会拿它去解一段明文 JSON。
func TestAPIEncryptDeliversHandlerErrors(t *testing.T) {
	kp := newTestKeyPair(t)

	r := gin.New()
	r.Use(Recover())
	r.Use(APIEncryptWithConfig(responseConfig(kp)))
	r.POST("/auth/login", func(c *gin.Context) {
		// 只登记错误、不写响应体 —— 这是本项目 handler 的标准错误路径。
		_ = c.Error(errs.New("用户名或密码错误"))
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
func responseConfig(kp testKeyPair) config.APIEncrypt {
	cfg := enabledConfig(kp)
	cfg.ResponseURLs = []string{"/auth/login"}
	return cfg
}

// 响应加密：body 应变成密文，密钥头应能用私钥解开。
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

// 响应密钥必须每次请求都不同。
//
// 一次性密钥是 ECB 在这里尚可接受的前提：密钥复用会让密文成为可跨请求
// 比对的指纹。
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

// 未命中 responseUrls 的路径响应不该被加密。
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

// 密钥头必须出现在 Access-Control-Expose-Headers 里，否则跨域下前端读不到它。
//
// 且**不能顶掉** CORS 已经写进去的 X-Request-Id —— 那会让前端拿不到 traceId、
// 无法与服务端日志对账（见 trace.go 与 cors.go 的配套说明）。
func TestAPIEncryptExposesKeyHeaderWithoutClobbering(t *testing.T) {
	kp := newTestKeyPair(t)

	r := gin.New()
	r.Use(Recover())
	r.Use(CORS()) // 会写 Access-Control-Expose-Headers: X-Request-Id
	r.Use(TraceID())
	r.Use(APIEncryptWithConfig(responseConfig(kp)))
	r.POST("/auth/login", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": 1}) })

	header, body := clientEncrypt(t, `{"a":1}`, "1234567890123456", &kp.priv.PublicKey)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(config.DefaultAPIEncryptHeader, header)
	// 触发 CORS 写跨域头。
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

// Content-Length 必须按密文长度重写。
//
// 不改的话客户端会按明文长度截断密文，解出来是一段残缺数据。
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
