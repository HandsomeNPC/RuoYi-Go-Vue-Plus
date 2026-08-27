package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// msgDecryptFailed 解密失败的统一文案。
const msgDecryptFailed = "请求解密失败"

// msgEncryptRequired 命中强制加密清单但未携带密钥头时的文案。
const msgEncryptRequired = "没有访问权限，请联系管理员授权"

// APIEncrypt 接口加解密中间件，配置取自 config.Get()。
func APIEncrypt() gin.HandlerFunc {
	return APIEncryptWithConfig(config.Get().APIEncrypt)
}

// APIEncryptWithConfig 接口加解密中间件。必须注册在 RepeatableBody 之前。
func APIEncryptWithConfig(cfg config.APIEncryptConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	header := cfg.HeaderFlag
	if header == "" {
		header = config.DefaultAPIEncryptHeader
	}

	// 密钥在启动期解析一次并捕获进闭包。
	privateKey, err := encrypt.ParseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		panic("middleware: apiEncrypt 私钥解析失败: " + err.Error())
	}
	var publicKey *rsa.PublicKey
	if len(cfg.ResponseURLs) > 0 {
		if publicKey, err = encrypt.ParseRSAPublicKey(cfg.PublicKey); err != nil {
			panic("middleware: apiEncrypt 公钥解析失败: " + err.Error())
		}
	}

	maxSize := cfg.MaxBodySize
	if maxSize <= 0 {
		maxSize = config.DefaultAPIEncrypt().MaxBodySize
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 只解密 PUT / POST。
		if c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPost {
			headerValue := strings.TrimSpace(c.GetHeader(header))
			switch {
			case headerValue != "":
				// 只看头、不看清单。
				if !decryptRequest(c, headerValue, privateKey, maxSize) {
					return
				}
			case MatchAnyPath(path, cfg.RequestURLs):
				// 命中强制加密清单却没带密钥头 → 403 拒绝。
				log.Printf("[encrypt]%s 请求地址'%s',要求加密传输但未携带 %s 头",
					logTracePrefix(c), path, header)
				_ = c.Error(errs.New(response.CodeForbidden, msgEncryptRequired, ""))
				c.Abort()
				return
			}
		}

		if !MatchAnyPath(path, cfg.ResponseURLs) {
			c.Next()
			return
		}
		encryptResponse(c, header, publicKey)
	}
}

// decryptRequest 解密请求体并换回可读的明文 Body，返回是否应继续处理。
func decryptRequest(c *gin.Context, headerValue string, key *rsa.PrivateKey, maxSize int64) bool {
	// 与 body.go 同样的两级限流：ContentLength 先挡一手，但仍要靠 LimitReader 兜住。
	if c.Request.ContentLength > maxSize {
		rejectOversizedBody(c, c.Request.ContentLength, maxSize)
		return false
	}
	if c.Request.Body == nil {
		failDecrypt(c, "请求体为空")
		return false
	}

	// 多读 1 字节以区分「刚好到上限」与「已超限」。
	cipherBody, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSize+1))
	if err != nil {
		log.Printf("[encrypt]%s 请求地址'%s',读取请求体失败: %v",
			logTracePrefix(c), c.Request.URL.Path, err)
		_ = c.Error(err)
		c.Abort()
		return false
	}
	if int64(len(cipherBody)) > maxSize {
		rejectOversizedBody(c, int64(len(cipherBody)), maxSize)
		return false
	}

	// 第一层：RSA 私钥解出 base64 编码的 AES 密钥。
	decryptedHeader, err := encrypt.DecryptByRSA(headerValue, key)
	if err != nil {
		failDecrypt(c, "RSA 解密 AES 秘钥失败: "+err.Error())
		return false
	}
	// 第二层：再 base64 解一次才是真正的 AES 密钥。
	aesPassword, err := encrypt.DecryptByBase64(decryptedHeader)
	if err != nil {
		failDecrypt(c, "AES 秘钥 base64 解码失败: "+err.Error())
		return false
	}

	// 第三层：用这个密钥解请求体。
	plain, err := encrypt.DecryptByAES(string(bytes.TrimSpace(cipherBody)), aesPassword)
	if err != nil {
		failDecrypt(c, "AES 解密请求体失败: "+err.Error())
		return false
	}

	body := []byte(plain)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	// Content-Type 必须改成 application/json，让下游认识明文。
	c.Request.Header.Set("Content-Type", ContentTypeJSON)

	return true
}

// failDecrypt 记录真实原因并回统一文案。
func failDecrypt(c *gin.Context, detail string) {
	log.Printf("[encrypt]%s 请求地址'%s',解密失败: %s",
		logTracePrefix(c), c.Request.URL.Path, detail)
	_ = c.Error(errs.New(0, msgDecryptFailed, ""))
	c.Abort()
}

// encryptResponse 缓冲 handler 的响应体，结束后整体 AES 加密再写出。
func encryptResponse(c *gin.Context, header string, key *rsa.PublicKey) {
	aesPassword, err := encrypt.GenerateAESPassword()
	if err != nil {
		failEncrypt(c, "生成 AES 秘钥失败: "+err.Error())
		return
	}
	encryptedKey, err := encrypt.EncryptByRSA(encrypt.EncryptByBase64(aesPassword), key)
	if err != nil {
		failEncrypt(c, "RSA 加密 AES 秘钥失败: "+err.Error())
		return
	}

	// 密钥头必须在 c.Next() 之前写，body 一开始输出头就发出去了。
	c.Writer.Header().Set(header, encryptedKey)
	// 用 Add 而非 Set：CORS 中间件可能已写过这个头。
	c.Writer.Header().Add("Access-Control-Expose-Headers", header)

	original := c.Writer
	buf := &encryptWriter{ResponseWriter: original, header: header}
	c.Writer = buf

	c.Next()

	// 必须还原 c.Writer，否则 Recover 在本函数返回后渲染错误会落进已用完的缓冲区。
	c.Writer = original

	// handler 没写响应：要么真没响应体，要么走了 c.Error 由 Recover 后续渲染。
	// 后者那份响应无法加密（发生在本中间件之外），必须撤掉密钥头。
	if buf.body.Len() == 0 {
		c.Writer.Header().Del(header)
		return
	}

	cipherText, err := encrypt.EncryptByAES(buf.body.String(), aesPassword)
	if err != nil {
		// 加密失败绝不能退回写明文，直接丢弃响应体并回错误。
		log.Printf("[encrypt]%s 请求地址'%s',响应加密失败,已丢弃响应体: %v",
			logTracePrefix(c), c.Request.URL.Path, err)
		buf.discardAndFail()
		return
	}
	buf.writeEncrypted(cipherText)
}

// failEncrypt 响应加密的前置步骤失败时回错误，此时 handler 还没执行。
func failEncrypt(c *gin.Context, detail string) {
	log.Printf("[encrypt]%s 请求地址'%s',响应加密准备失败: %s",
		logTracePrefix(c), c.Request.URL.Path, detail)
	_ = c.Error(errs.New(0, "响应加密失败", ""))
	c.Abort()
}

// encryptWriter 缓冲响应体的 gin.ResponseWriter 包装。
type encryptWriter struct {
	gin.ResponseWriter
	body bytes.Buffer

	header string
}

// Write 把响应体写进缓冲区而不是直接发出。
func (w *encryptWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

// WriteString 同 Write，gin 的 c.String 等走这个方法。
func (w *encryptWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

// writeEncrypted 把密文写进真正的响应。
func (w *encryptWriter) writeEncrypted(cipherText string) {
	h := w.ResponseWriter.Header()
	// 长度按密文重算。
	h.Set("Content-Length", strconv.Itoa(len(cipherText)))
	// 密文是 base64 纯文本，不再是 JSON。
	h.Set("Content-Type", "text/plain;charset=utf-8")
	_, _ = w.ResponseWriter.WriteString(cipherText)
}

// discardAndFail 丢弃已缓冲的明文并写一个加密失败的响应。
func (w *encryptWriter) discardAndFail() {
	w.body.Reset()
	h := w.ResponseWriter.Header()
	// 密钥头必须撤掉：响应体已不是用它加密的。
	h.Del(w.header)
	h.Set("Content-Type", "application/json; charset=utf-8")
	body := []byte(`{"code":500,"msg":"响应加密失败","data":null}`)
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = w.ResponseWriter.Write(body)
}
