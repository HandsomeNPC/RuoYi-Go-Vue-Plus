// api_encrypt.go 接口加解密的注解（装饰器）层，对照 sa-token-go 的注解形式
// （sagin.CheckPermission）与 Java @ApiEncrypt + CryptoFilter。
// 初始化对照 redis.Init：encrypt.Init() 无参，自读 config.Get().APIEncrypt，
// 解析密钥并设包级全局；路由用包级 encrypt.ApiEncrypt() / ApiEncryptWithResponse()。
package encrypt

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

const (
	defaultHeader     = "encrypt-key"    // 默认密钥头名
	defaultMaxBodyAge = 10 * 1024 * 1024 // 默认密文上限 10MB
)

// Crypto 接口加解密器，持有解析后的密钥。
type Crypto struct {
	header  string
	priv    *rsa.PrivateKey
	pub     *rsa.PublicKey
	maxSize int64
}

// New 解析配置构造 Crypto。!Enabled 时返回 no-op（注解直通）。对照 redis.New。
func New(cfg config.APIEncryptConfig) (*Crypto, error) {
	if !cfg.Enabled {
		return &Crypto{}, nil
	}
	priv, err := ParseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	var pub *rsa.PublicKey
	if cfg.PublicKey != "" {
		pub, err = ParseRSAPublicKey(cfg.PublicKey)
		if err != nil {
			return nil, err
		}
	}
	header := cfg.HeaderFlag
	if header == "" {
		header = defaultHeader
	}
	return &Crypto{header: header, priv: priv, pub: pub, maxSize: defaultMaxBodyAge}, nil
}

// 包级默认实例（对照 redis.defaultClient）。
var (
	mu            sync.RWMutex
	defaultCrypto *Crypto
)

// Init 按 config.Get().APIEncrypt 构造并设包级默认实例。对照 redis.Init。
// 必须在 config.Load 之后调用。密钥解析失败直接 panic（启动期 fail-fast）。
func Init() {
	c := config.Get()
	cfg := c.APIEncrypt
	crypto, err := New(cfg)
	if err != nil {
		panic(fmt.Errorf("encrypt: 初始化失败: %w", err))
	}
	mu.Lock()
	defaultCrypto = crypto
	mu.Unlock()
	log.Printf("[%s] encrypt 已就绪: enabled=%t header=%s", c.Server.Name, cfg.Enabled, crypto.header)
}

// getCrypto 返回包级默认实例，未调用 Init 会 panic。对照 redis.Client。
func getCrypto() *Crypto {
	mu.RLock()
	c := defaultCrypto
	mu.RUnlock()
	if c == nil {
		panic("encrypt: 尚未初始化，请先调用 encrypt.Init")
	}
	return c
}

// ApiEncrypt 请求解密注解（包级，对照 sagin.CheckPermission）。
// POST/PUT 带 header 则解密请求体，未带则 403。
func ApiEncrypt() gin.HandlerFunc {
	return func(c *gin.Context) { getCrypto().run(c, false) }
}

// ApiEncryptWithResponse 请求解密 + 响应加密注解（对照 Java @ApiEncrypt(response = true)）。
func ApiEncryptWithResponse() gin.HandlerFunc {
	return func(c *gin.Context) { getCrypto().run(c, true) }
}

// run 执行加解密，对照 Java CryptoFilter.doFilter。
func (e *Crypto) run(c *gin.Context, withResponse bool) {
	if e.priv == nil {
		// 未启用（!Enabled）：注解直通，不做任何加解密。
		c.Next()
		return
	}
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
		headerValue := strings.TrimSpace(c.GetHeader(e.header))
		if headerValue == "" {
			// 命中加密注解却未带密钥头 → 403。
			log.Printf("[encrypt] %s %s 要求加密传输但未携带 %s 头",
				c.Request.Method, c.Request.URL.Path, e.header)
			_ = c.Error(errs.New(response.CodeForbidden, "没有访问权限，请联系管理员授权", ""))
			c.Abort()
			return
		}
		if !e.decryptRequest(c, headerValue) {
			return
		}
	}
	if !withResponse {
		c.Next()
		return
	}
	e.encryptResponse(c)
}

// decryptRequest 解密请求体并换回明文 body，对照 Java DecryptRequestBodyWrapper。
func (e *Crypto) decryptRequest(c *gin.Context, headerValue string) bool {
	if c.Request.ContentLength > e.maxSize {
		e.fail(c, "请求体超限")
		return false
	}
	if c.Request.Body == nil {
		e.fail(c, "请求体为空")
		return false
	}
	cipherBody, err := io.ReadAll(io.LimitReader(c.Request.Body, e.maxSize+1))
	if err != nil {
		e.fail(c, "读取请求体失败: "+err.Error())
		return false
	}
	if int64(len(cipherBody)) > e.maxSize {
		e.fail(c, "请求体超限")
		return false
	}
	// RSA 私钥解出 base64 编码的 AES 密钥。
	decryptedHeader, err := DecryptByRSA(headerValue, e.priv)
	if err != nil {
		e.fail(c, "RSA 解密 AES 秘钥失败: "+err.Error())
		return false
	}
	// 再 base64 解一次才是真正的 AES 密钥。
	aesPassword, err := DecryptByBase64(decryptedHeader)
	if err != nil {
		e.fail(c, "AES 秘钥 base64 解码失败: "+err.Error())
		return false
	}
	// AES 解密请求体。
	plain, err := DecryptByAES(string(bytes.TrimSpace(cipherBody)), aesPassword)
	if err != nil {
		e.fail(c, "AES 解密请求体失败: "+err.Error())
		return false
	}
	body := []byte(plain)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// 同步更新 BodyBytesKey：RepeatableBody 在本注解之前已缓存密文，此处覆盖为明文，
	// 让下游用 middleware.BodyBytes 取到的是明文。
	c.Set(gin.BodyBytesKey, body)
	return true
}

// encryptResponse 缓冲响应体，结束后整体加密再写出，对照 Java EncryptResponseBodyWrapper。
func (e *Crypto) encryptResponse(c *gin.Context) {
	if e.pub == nil {
		log.Printf("[encrypt] %s 配置了响应加密但未提供公钥", c.Request.URL.Path)
		_ = c.Error(errs.New(0, "响应加密失败", ""))
		c.Abort()
		return
	}
	aesPassword, err := GenerateAESPassword()
	if err != nil {
		e.failResponse(c, "生成 AES 秘钥失败: "+err.Error())
		return
	}
	encryptedKey, err := EncryptByRSA(EncryptByBase64(aesPassword), e.pub)
	if err != nil {
		e.failResponse(c, "RSA 加密 AES 秘钥失败: "+err.Error())
		return
	}
	// 密钥头必须在 c.Next() 之前写，body 一开始输出头就发出去了。
	c.Writer.Header().Set(e.header, encryptedKey)
	c.Writer.Header().Add("Access-Control-Expose-Headers", e.header)

	original := c.Writer
	buf := &cryptoWriter{ResponseWriter: original}
	c.Writer = buf
	c.Next()
	// 还原 c.Writer，否则 Recover 在本函数返回后渲染错误会落进已用完的缓冲区。
	c.Writer = original

	if buf.body.Len() == 0 {
		// handler 没写响应（可能走了 c.Error 由 Recover 渲染），撤掉密钥头。
		c.Writer.Header().Del(e.header)
		return
	}
	cipherText, err := EncryptByAES(buf.body.String(), aesPassword)
	if err != nil {
		// 加密失败绝不能退回写明文，丢弃响应体并回错误。
		log.Printf("[encrypt] %s 响应加密失败,已丢弃响应体: %v", c.Request.URL.Path, err)
		buf.discardAndFail(e.header)
		return
	}
	buf.writeEncrypted(cipherText)
}

// fail 记录明细并回请求解密失败的统一文案。
func (e *Crypto) fail(c *gin.Context, detail string) {
	log.Printf("[encrypt] %s %s 解密失败: %s", c.Request.Method, c.Request.URL.Path, detail)
	_ = c.Error(errs.New(0, "请求解密失败", ""))
	c.Abort()
}

// failResponse 响应加密前置步骤失败时回错误，此时 handler 还没执行。
func (e *Crypto) failResponse(c *gin.Context, detail string) {
	log.Printf("[encrypt] %s 响应加密准备失败: %s", c.Request.URL.Path, detail)
	_ = c.Error(errs.New(0, "响应加密失败", ""))
	c.Abort()
}

// cryptoWriter 缓冲响应体的 gin.ResponseWriter 包装。
type cryptoWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *cryptoWriter) Write(b []byte) (int, error)       { return w.body.Write(b) }
func (w *cryptoWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }

// writeEncrypted 把密文写进真正的响应。
func (w *cryptoWriter) writeEncrypted(cipherText string) {
	h := w.ResponseWriter.Header()
	h.Set("Content-Length", strconv.Itoa(len(cipherText))) // 长度按密文重算
	h.Set("Content-Type", "text/plain;charset=utf-8")      // 密文是 base64 纯文本
	_, _ = w.ResponseWriter.WriteString(cipherText)
}

// discardAndFail 丢弃已缓冲的明文并写一个加密失败的响应。
func (w *cryptoWriter) discardAndFail(header string) {
	w.body.Reset()
	h := w.ResponseWriter.Header()
	h.Del(header) // 密钥头必须撤掉：响应体已不是用它加密的
	h.Set("Content-Type", "application/json; charset=utf-8")
	body := []byte(`{"code":500,"msg":"响应加密失败","data":null}`)
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = w.ResponseWriter.Write(body)
}
