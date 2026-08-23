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
//
// **所有解密失败共用这一句**，不区分是 RSA 阶段失败、AES 密钥长度不对、
// 还是 PKCS#7 填充校验没过。这是刻意的：把失败原因区分开就等于提供一个
// padding oracle —— 攻击者可以拿同一段密文反复试探，靠「填充错」与
// 「解密错」两种不同回复逐字节还原明文（Vaudenay 攻击）。
// 真实原因进日志，前端只拿这一句。
//
// 文案本身不走 i18n 词条：原项目的 CryptoFilter 也是硬编码中文，
// 且这条路径上还没有词条（pkg/i18n 的 54 条是从 messages.properties 搬的，
// 里面没有加解密相关的）。要加应当先在原项目侧确认词条 key。
const msgDecryptFailed = "请求解密失败"

// msgEncryptRequired 命中强制加密清单但未携带密钥头时的文案。
//
// 对齐 CryptoFilter 里那句「没有访问权限，请联系管理员授权」+ 403。
// 文案照抄是为了不改前端已有的提示，尽管它其实相当误导人 ——
// 真实原因是「这个接口要求加密传输，而你发的是明文」，与授权无关。
const msgEncryptRequired = "没有访问权限，请联系管理员授权"

// APIEncrypt 接口加解密中间件，配置取自 config.Get()。
func APIEncrypt() gin.HandlerFunc {
	return APIEncryptWithConfig(config.Get().Middleware.APIEncrypt)
}

// APIEncryptWithConfig 接口加解密中间件，对应原项目
// encrypt/filter/CryptoFilter.java + DecryptRequestBodyWrapper.java
// + EncryptResponseBodyWrapper.java。
//
// # 协议
//
// 请求方向（前端 → 服务端），两层套嵌：
//
//	encrypt-key 头 = base64(RSA_公钥加密( base64( AES明文密钥 ) ))
//	请求体         = base64(AES_ECB加密( JSON 明文 ))
//
// 注意 AES 密钥被 base64 **套了两层**：里层那层是原项目 EncryptUtils
// 的 encryptByBase64/decryptByBase64，外层是 RSA 密文本身的 base64。
// 看着多余，但它是协议的一部分（前端照此实现），少一层就对不上。
//
// 响应方向（服务端 → 前端）与之对称，AES 密钥换成服务端每次请求新生成的
// 一次性密钥、用**前端的公钥**（配置里的 publicKey）加密后放同名头。
//
// # 注册顺序：必须在 RepeatableBody 之前
//
//	Recover → CORS → TraceID → APIEncrypt → RepeatableBody → AccessLog → XSS → I18n → Auth
//
// 依据是 Java 侧的 Filter order：CryptoFilter 是 HIGHEST_PRECEDENCE，
// 而 RepeatableFilter 未指定 order（= LOWEST）。关键证据在
// DecryptRequestBodyWrapper.getContentType() —— 它**恒返回 application/json**，
// 于是 RepeatableFilter 的 startsWith 判定通过、会在解密包装外面再包一层：
//
//	RepeatedlyRequestWrapper( DecryptRequestBodyWrapper( 原始 request ) )
//
// 也就是说 Java 侧拦截器读到的是**解密后的明文**，那边的脱敏是真的作用在
// 明文上。顺序摆反的后果：
//
//   - 放在 AccessLog 之后 → 日志里永远只有 base64 密文（最长 4000 字符的
//     乱码、零诊断价值），脱敏形同虚设，且 handler 绑不到参数（body 已被吃掉）。
//   - 放在 RepeatableBody 之前（本实现）→ 日志是明文、脱敏正常生效。
//     代价是**明文密码会流进 jsonParamLog**，那条路径的 removeSensitiveFields
//     必须靠得住 —— 这正是 logger.go 里 rawParamLog 有意偏离 Java、
//     对非法 JSON 也要做敏感字段探测的原因。四个 @ApiEncrypt 接口全都在传密码。
//
// # 相对 Java 的偏差
//
// | 位置       | 偏差                              | 原因                                                        |
// |------------|-----------------------------------|-------------------------------------------------------------|
// | 命中方式   | 配置路径清单，非注解反查          | Go 无注解；Java 靠 RequestMappingHandlerMapping 查 @ApiEncrypt |
// | 失败文案   | 全部折叠成一句                    | 区分失败原因 = 提供 padding oracle                          |
// | 失败状态码 | 恒 200 + 业务码                   | 走 c.Error 由 Recover 统一渲染，与其余接口一致               |
// | 密钥解析   | 启动期一次                        | Java 每请求重新 KeyFactory.generatePublic 解析 ASN.1        |
// | 体积上限   | 复用 RepeatableBody.MaxBodySize   | Java 侧无上限（IoUtil.readBytes 读到底）                    |
// | 响应加密   | 按路径清单，且默认关闭            | 原项目 4 处 @ApiEncrypt 全是 response=false，从未启用       |
func APIEncryptWithConfig(cfg config.APIEncrypt) gin.HandlerFunc {
	if !cfg.Enabled {
		// 关闭时返回空操作而非让 Register 跳过：这样「关闭」和「启用但请求
		// 未加密」走同一条代码路径，少一种只在特定配置下才跑到的分支。
		return func(c *gin.Context) { c.Next() }
	}

	header := cfg.HeaderFlag
	if header == "" {
		header = config.DefaultAPIEncryptHeader
	}

	// 密钥在启动期解析一次并捕获进闭包。config.APIEncrypt.validate 已经
	// 解析过一遍（好让格式错误在 Load 时就报出来），这里再解析是因为
	// 那边只做校验、不留结果。两次都失败于同一个输入，不会出现不一致。
	//
	// 解析失败直接 panic：走到这里说明调用方绕过了 config.Load 的校验
	// （用 APIEncryptWithConfig 显式传了配置），那是启动期的编排错误，
	// 与 config.Get() 未初始化时 panic 同源 —— 不该留到运行时。
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

	// 密文经 base64 后比明文大约 4/3，与 RepeatableBody.MaxBodySize 同一量级，
	// 默认值也取同一个常量；但**读的是自己的配置项**而非那边的 ——
	// WithConfig 系列的约定是「只用调用方显式传入的配置」，
	// 在这里回头调 config.Get() 会让测试与不走 Load 的调用方直接 panic。
	maxSize := cfg.MaxBodySize
	if maxSize <= 0 {
		maxSize = config.DefaultMiddleware().APIEncrypt.MaxBodySize
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 只有 PUT / POST 才解密，对齐 CryptoFilter 里那个
		// HttpMethod.PUT.matches || HttpMethod.POST.matches 判断。
		// 带 body 的 PATCH 在原项目里同样不解密 —— 这是原项目的口径，
		// 不在这里擅自扩大（扩大意味着一条 Java 侧没有的行为分支）。
		if c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPost {
			headerValue := strings.TrimSpace(c.GetHeader(header))
			switch {
			case headerValue != "":
				// 只看头、不看清单，对齐 Java：那边 headerValue 非空就解密，
				// 与方法上有没有 @ApiEncrypt 无关。
				if !decryptRequest(c, headerValue, privateKey, maxSize) {
					return
				}
			case MatchAnyPath(path, cfg.RequestURLs):
				// 对齐 CryptoFilter：命中强制加密清单却没带密钥头 → 403 拒绝。
				// 这条分支拦的是「本该加密的接口收到了明文密码」。
				log.Printf("[encrypt]%s 请求地址'%s',要求加密传输但未携带 %s 头",
					logTracePrefix(c), path, header)
				_ = c.Error(errs.NewCode(response.CodeForbidden, msgEncryptRequired))
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

// decryptRequest 解密请求体并换回一个可读的明文 Body，返回是否应继续处理。
//
// 失败时已经登记错误并 Abort，调用方直接 return 即可。
func decryptRequest(c *gin.Context, headerValue string, key *rsa.PrivateKey, maxSize int64) bool {
	// 与 body.go 同样的两级限流：ContentLength 先挡一手（省掉白读），
	// 但它不可信也不总有值（chunked 是 -1），仍要靠 LimitReader 兜住。
	if c.Request.ContentLength > maxSize {
		rejectOversizedBody(c, c.Request.ContentLength, maxSize)
		return false
	}
	if c.Request.Body == nil {
		// 带密钥头却没有 body：解不出任何东西，当解密失败处理。
		// 不静默放行 —— 那会让一个畸形请求以「参数为空」的面貌到达 handler。
		failDecrypt(c, "请求体为空")
		return false
	}

	// 多读 1 字节以区分「刚好到上限」与「已超限」，理由同 body.go。
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

	// 第一层：RSA 私钥解出「base64 编码的 AES 密钥」。
	decryptedHeader, err := encrypt.DecryptByRSA(headerValue, key)
	if err != nil {
		failDecrypt(c, "RSA 解密 AES 秘钥失败: "+err.Error())
		return false
	}
	// 第二层：再 base64 解一次才是真正的 AES 密钥。这层套嵌是协议的一部分，
	// 对应 EncryptUtils.decryptByBase64(decryptAes)。
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
	// Content-Type 必须改成 application/json，这是让下游认识明文的关键 ——
	// 对齐 DecryptRequestBodyWrapper.getContentType() 恒返回 application/json。
	// 加密请求的原始 Content-Type 多半是 text/plain（前端发的是 base64 串），
	// 不改的话 RepeatableBody 不会缓存它、AccessLog 打不出 json 参数、
	// XSS 跳过 body 清洗、handler 的 ShouldBindJSON 也会拒绝。
	c.Request.Header.Set("Content-Type", ContentTypeJSON)

	return true
}

// failDecrypt 记录真实原因并回统一文案。
//
// detail 只进日志：它区分了 RSA / base64 / AES / 填充四类失败，
// 回给前端就成了 padding oracle（见 msgDecryptFailed）。
//
// 走 c.Error + Abort 而非自己写响应，与 body.go 的 rejectOversizedBody 一致：
// 由最外层 Recover 统一渲染成 response.R，HTTP 200 + 业务码在 body 里。
func failDecrypt(c *gin.Context, detail string) {
	log.Printf("[encrypt]%s 请求地址'%s',解密失败: %s",
		logTracePrefix(c), c.Request.URL.Path, detail)
	_ = c.Error(errs.New(msgDecryptFailed))
	c.Abort()
}

// encryptResponse 缓冲 handler 的响应体，结束后整体 AES 加密再写出。
//
// 对应 EncryptResponseBodyWrapper：Java 用 HttpServletResponseWrapper
// 把输出流换成 ByteArrayOutputStream，Go 里对应换掉 c.Writer。
//
// **原项目从未启用这条路径**（4 处 @ApiEncrypt 全是 response=false），
// 所以没有可对照验证的线上行为，仅按 EncryptResponseBodyWrapper 的代码复刻。
func encryptResponse(c *gin.Context, header string, key *rsa.PublicKey) {
	// 每次请求新生成一次性 AES 密钥，对齐 generateAesPassword。
	// 一次性是 ECB 在这里尚可接受的前提（见 encrypt.ecbEncrypt 的说明）。
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

	// 密钥头必须在 c.Next() **之前**写：body 一开始输出，头就已经发出去了，
	// 事后 Set 会静默失效（与 trace.go / i18n.go 同一条纪律）。
	// 这里能提前写是因为密钥与响应内容无关 —— 先生成密钥、再加密内容。
	c.Writer.Header().Set(header, encryptedKey)
	// 跨域下前端 JS 默认读不到自定义响应头，必须显式暴露。
	// 对齐 EncryptResponseBodyWrapper 里那行 addHeader("Access-Control-Expose-Headers", ...)。
	// 用 Add 而非 Set：CORS 中间件可能已经写过这个头（X-Request-Id），
	// Set 会把它顶掉，前端就拿不到 traceId 了。
	c.Writer.Header().Add("Access-Control-Expose-Headers", header)

	original := c.Writer
	buf := &encryptWriter{ResponseWriter: original, header: header}
	c.Writer = buf

	c.Next()

	// **必须还原 c.Writer**，否则后续写入会落进这个已经用完的缓冲区里被丢弃。
	// 具体到本项目：handler 只 c.Error(err) 而不写 body 时，渲染错误响应的是
	// 最外层的 Recover —— 那发生在本函数返回**之后**，此时若 c.Writer 还是
	// buf，那份错误响应就凭空消失了，客户端收到一个 200 空响应。
	// 由 TestAPIEncryptDeliversHandlerErrors 锁住。
	c.Writer = original

	// handler 什么都没写。两种情形：
	//   - 真的没有响应体（204 之类）
	//   - handler 走了 c.Error，响应将由 Recover 在本函数返回后渲染
	// 后者那份响应**没法加密** —— Recover 在本中间件之外，它写响应时
	// 本函数早已返回。此时必须撤掉密钥头：留着会让前端拿它去解一段明文 JSON，
	// 得到一句「解密失败」而非服务端真正想传达的错误。
	//
	// 这是相对 Java 的一处**有意偏差**：那边 CryptoFilter 在 filter 链内部，
	// 异常经 handlerExceptionResolver 渲染后仍会被 wrapper 缓冲并加密；
	// Go 侧 Recover 必须在最外层（要兜住所有中间件自身的 panic），
	// 顺序上无法两全。取「错误能送达但不加密」而非「加密但送不到」。
	if buf.body.Len() == 0 {
		c.Writer.Header().Del(header)
		return
	}

	cipherText, err := encrypt.EncryptByAES(buf.body.String(), aesPassword)
	if err != nil {
		// 已经缓冲了明文但加密失败 —— 绝不能退回去写明文（那正是要防的），
		// 直接丢弃响应体、回一个不含数据的错误。
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
	_ = c.Error(errs.New("响应加密失败"))
	c.Abort()
}

// encryptWriter 缓冲响应体的 gin.ResponseWriter 包装。
//
// 内嵌 gin.ResponseWriter 而非 http.ResponseWriter：c.Writer 是前者，
// 内嵌它才能自动继承 Status/Size/Written/Hijack/Flush/Pusher 等方法，
// 只覆写真正要改的 Write / WriteString。
//
// **有意不覆写 Flush**：内嵌的 Flush 会把「还没加密的空内容」推给客户端并
// 锁定响应头。流式接口（SSE、大文件下载）与整体加密在语义上不兼容 ——
// 加密要求先看到完整内容，流式要求边产边发。这类接口不该出现在
// responseUrls 里；真的出现了，表现是 Flush 提前发头、随后的加密内容
// 附在后面，报文错乱。没有为此加保护是因为清单是人工配的，
// 而加了保护反而会掩盖「把 SSE 配进加密清单」这个配置错误。
type encryptWriter struct {
	gin.ResponseWriter
	body bytes.Buffer

	// header 密钥头名，加密失败时要把它撤掉（见 discardAndFail）。
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
	// 长度按密文重算，对齐 setContentLengthLong。不改的话，
	// handler 写的明文长度会留在头里，客户端按它截断密文。
	h.Set("Content-Length", strconv.Itoa(len(cipherText)))
	// 密文是 base64 纯文本，不再是 JSON。留着 application/json 会让
	// 浏览器 devtools 和中间代理尝试解析它、报一堆无意义的解析错。
	h.Set("Content-Type", "text/plain;charset=utf-8")
	_, _ = w.ResponseWriter.WriteString(cipherText)
}

// discardAndFail 丢弃已缓冲的明文并写一个加密失败的响应。
//
// 不走 c.Error：handler 已经执行完，Recover 那一环的 c.Writer.Written()
// 判断此时不可靠（我们截住了写入，真实 writer 尚未被写过），
// 交回去会得到两份响应体。这里直接写完整的 response.R。
func (w *encryptWriter) discardAndFail() {
	w.body.Reset()
	h := w.ResponseWriter.Header()
	// 密钥头必须撤掉：响应体已不是用它加密的，留着会让前端拿它去解一段明文 JSON，
	// 得到的是一句解密失败而非我们想传达的「响应加密失败」。
	h.Del(w.header)
	h.Set("Content-Type", "application/json; charset=utf-8")
	body := []byte(`{"code":500,"msg":"响应加密失败","data":null}`)
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = w.ResponseWriter.Write(body)
}
