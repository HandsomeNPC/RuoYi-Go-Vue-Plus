package middleware

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/errs"
)

// ContentTypeJSON JSON 请求的 content-type 前缀。
//
// 用前缀匹配而非全等，因为实际请求多半带参数（application/json;charset=UTF-8），
// 对齐 RepeatableFilter 里的 startsWithIgnoreCase(contentType, APPLICATION_JSON_VALUE)。
const ContentTypeJSON = "application/json"

// defaultMaxBodySize 默认缓存上限 10MB，与 Java 侧 spring.servlet.multipart.max-file-size
// 取同一个数量级（application.yml:70）。
//
// 原项目这里**没有上限**：RepeatedlyRequestWrapper 用 IoUtil.readBytes 一次读完，
// 而 Tomcat 的 max-http-form-post-size 只管表单、不管 JSON。Java 侧能这么写是因为
// 它前面有 nginx 的 client_max_body_size 兜着；Go 侧仍然要自己设限 ——
// 无上限的 io.ReadAll 等于让调用方决定进程吃多少内存，一个几 GB 的 chunked
// 请求就能把进程 OOM 掉，这是纯粹的放大攻击（代价在服务端，成本在客户端）。
const defaultMaxBodySize = 10 << 20

// RepeatableBodyConfig 可重复读 body 的配置。
type RepeatableBodyConfig struct {
	// ContentTypes 需要缓存的 content-type 前缀（小写），大小写不敏感匹配。
	//
	// 默认只含 application/json，与 RepeatableFilter 一致。**不要**为了让日志
	// 打得更全而把 multipart/form-data 加进来：那会把上传的文件整个读进内存，
	// 原项目允许 10MB 单文件 / 20MB 单请求，并发几个就够把进程压垮。
	//
	// 表单请求（application/x-www-form-urlencoded）也不需要加：
	// net/http 的 ParseForm 会把解析结果缓存进 r.PostForm，
	// 后续中间件与 handler 读的是解析结果而不是 body，天然可重复。
	// 这与 Java 侧 AccessLog 走 getParameterMap() 而非读 body 是同一个道理。
	ContentTypes []string

	// MaxBodySize 允许缓存的最大字节数，超出则拒绝请求。<=0 表示用默认值。
	MaxBodySize int64
}

// DefaultRepeatableBodyConfig 返回默认配置。
func DefaultRepeatableBodyConfig() RepeatableBodyConfig {
	return RepeatableBodyConfig{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  defaultMaxBodySize,
	}
}

// RepeatableBody 可重复读请求体中间件，用默认配置。
func RepeatableBody() gin.HandlerFunc {
	return RepeatableBodyWithConfig(DefaultRepeatableBodyConfig())
}

// RepeatableBodyWithConfig 可重复读请求体中间件，对应原项目
// web/filter/RepeatableFilter.java + web/filter/RepeatedlyRequestWrapper.java。
//
// Java 侧的做法是把 request 换成一个包装类，其 getInputStream() 每次都基于
// 缓存的 byte[] 新建一个流。Go 里 c.Request.Body 是 io.ReadCloser，
// 读完即空，所以等价实现是「读出来，再塞一个新的 Reader 回去」。
//
// **必须注册在 AccessLog 之前**：body 是一次性的，日志中间件读完 handler
// 就绑不到参数了。这个顺序约束在 Java 侧是隐式的 ——
// PlusWebInvokeTimeInterceptor 显式判 `request instanceof RepeatedlyRequestWrapper`，
// 没被包装过就干脆不打参数（宁可少打日志，也不能把 body 吃掉）。
// Go 里同理：BodyBytes 取不到就别去读 c.Request.Body。
//
// 除日志外，阶段 3 的 @Log 操作日志、以及将来的验签 / 解密也都依赖它。
//
// 相比 Java 多做一件事：同时把 body 存进 gin.BodyBytesKey，
// 这样 handler 用 c.ShouldBindBodyWith 能直接复用缓存、多次绑定不同结构体，
// 不必自己去 BodyBytes 里捞。两个键存的是同一个底层数组，没有额外拷贝。
func RepeatableBodyWithConfig(cfg RepeatableBodyConfig) gin.HandlerFunc {
	maxSize := cfg.MaxBodySize
	if maxSize <= 0 {
		maxSize = defaultMaxBodySize
	}
	// 预先转小写，省掉每请求一次的 ToLower —— 配置在启动后不变。
	types := make([]string, 0, len(cfg.ContentTypes))
	for _, t := range cfg.ContentTypes {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			types = append(types, t)
		}
	}

	return func(c *gin.Context) {
		if !shouldBufferBody(c, types) {
			c.Next()
			return
		}

		// ContentLength 已经报了超限就不用读了，直接拒 —— 省掉白读 10MB。
		// 但它不可信也不总有值（chunked 编码是 -1），所以下面仍要限流读取。
		if c.Request.ContentLength > maxSize {
			rejectOversizedBody(c, c.Request.ContentLength, maxSize)
			return
		}

		// 多读 1 字节用来区分「刚好到上限」和「已经超了」：
		// LimitReader 读满就返回 EOF，不加这 1 字节的话，一个正好 maxSize+N
		// 的请求会被截断成 maxSize 后当作正常 body 交给 handler，
		// 变成一个 JSON 解析失败的谜之报错。
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSize+1))
		if err != nil {
			// 读不完通常是客户端中途断开。这里不静默：body 读失败意味着
			// 请求本身不完整，继续往下走只会让 handler 拿到半截 JSON。
			// 交给 Recover 兜底成系统异常（连接真断了写不出去也无妨）。
			log.Printf("[body]%s 请求地址'%s',读取请求体失败: %v",
				logTracePrefix(c), c.Request.URL.Path, err)
			_ = c.Error(err)
			c.Abort()
			return
		}
		if int64(len(body)) > maxSize {
			rejectOversizedBody(c, int64(len(body)), maxSize)
			return
		}

		// 关键一步：塞回一个可读的 Body，让后续的 c.ShouldBindJSON 照常工作。
		// NopCloser 是因为原始 Body 的 Close 由 net/http 的 server 负责，
		// 这里包一层不需要也不应该再关它。
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Set(gin.BodyBytesKey, body)

		c.Next()
	}
}

// BodyBytes 取已缓存的请求体，未缓存时返回 nil。
//
// 给 AccessLog、@Log 操作日志这类需要读 body 的中间件用。
//
// **拿不到就不要退回去读 c.Request.Body**：那会把 body 吃掉，
// handler 再绑参数就是空的。返回 nil 的正常情况有三种：
// 非 JSON 请求、body 为空、没挂 RepeatableBody 中间件。
// 对齐 Java 侧 `if (request instanceof RepeatedlyRequestWrapper)` 的判断 ——
// 没被包装过就跳过打参数，而不是硬读。
//
// 返回的是缓存本身而非副本，调用方**不要修改**它：
// 它与 c.Request.Body 共用底层数组，改了会让 handler 绑到被篡改的数据。
func BodyBytes(c *gin.Context) []byte {
	v, ok := c.Get(gin.BodyBytesKey)
	if !ok {
		return nil
	}
	body, _ := v.([]byte)
	return body
}

// shouldBufferBody 判断本次请求是否需要缓存 body。
func shouldBufferBody(c *gin.Context, types []string) bool {
	// Body 为 nil 出现在 GET 这类无体请求和部分单测里；
	// ContentLength 为 0 时读出来也是空切片，白花一次系统调用。
	// 两种情况都不设 BodyBytesKey —— 缺键与空 body 对 gin 而言等价
	// （都会让 ShouldBindBodyWith 报 EOF），少存一个键省得让人误以为缓存过。
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return false
	}

	// 不按请求方法过滤，对齐 RepeatableFilter —— 它只看 content-type。
	// （跳过 GET/DELETE 是 XssFilter 的行为，两个 filter 别搞混。）
	// 带 JSON body 的 DELETE 是合法的，按方法排除会让它读不到参数。
	ct := strings.ToLower(c.ContentType())
	for _, t := range types {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}

// rejectOversizedBody 拒绝超限的请求体。
//
// 走 c.Error + Abort 而不是自己写响应：错误由最外层的 Recover 统一渲染成
// response.R，这样返回结构和其余接口一致（HTTP 200 + 业务码在 body 里，
// 见 README「两条硬约束」）。这里不回真实 413 —— 前端拦截器只认 body.code。
//
// 具体尺寸只进日志：告诉调用方上限多少等于告诉它「贴着上限打」，
// 而这个数字对正常用户毫无用处（正常请求根本碰不到）。
func rejectOversizedBody(c *gin.Context, size, maxSize int64) {
	log.Printf("[body]%s 请求地址'%s',请求体超限: %d > %d 字节",
		logTracePrefix(c), c.Request.URL.Path, size, maxSize)
	_ = c.Error(errs.New("请求体超出大小限制"))
	c.Abort()
}
