package middleware

import "github.com/gin-gonic/gin"

// Register 按固定顺序注册全部全局中间件，各中间件的配置取自 config.Get()，
// 因此**必须先 config.Load 再调用本函数**，否则 Get() 会 panic。
//
// 抽成一个函数而非让各进程入口自己 r.Use：拆进程后同一个请求经 nginx
// 落到哪个进程是不定的，两边链路不同会让同一次调用表现出不同的清洗/脱敏
// 行为，而那种差异极难从现象反推。逐字复制的两段代码迟早分叉，
// 收到一处则物理上消除了这种可能。
//
// 调用方仍要用 gin.New() 而非 gin.Default()：后者自带的 gin.Recovery()
// 只写 500 空响应，与 Recover() 职责重叠且不输出 response.R。
//
// 顺序见 README「注册顺序」一节，五条硬约束在下面逐条标注，改动前先读那里。
func Register(r gin.IRoutes) {
	// Recover 放最外层，才能兜住后续中间件自身的 panic。
	r.Use(Recover())
	// CORS 必须在鉴权之前：预检是 OPTIONS 且不带 token，
	// 先过鉴权会被 401，浏览器就拿不到跨域头了。
	r.Use(CORS())
	// TraceID 紧跟 CORS：越靠前，越多日志能带上链路 id。
	// 放在 CORS 之后是因为跨域预检会被 CORS 就地终止，不进业务也不需要 id。
	r.Use(TraceID())
	// APIEncrypt 必须在 RepeatableBody 之前：解密后才把 body 换成明文，
	// 顺序反了 AccessLog 只能看到 base64 密文（脱敏形同虚设），
	// 且 body 已被读走、handler 绑不到参数。
	// 依据是 Java 侧 CryptoFilter 的 order = HIGHEST_PRECEDENCE，
	// 完整推导（含 DecryptRequestBodyWrapper 恒返回 application/json 那条证据）
	// 见 README「注册顺序」一节与 crypto.go 的函数注释。
	r.Use(APIEncrypt())
	// RepeatableBody 必须在 AccessLog 之前：body 是一次性的 io.ReadCloser，
	// 日志中间件读完 handler 就绑不到参数了。
	r.Use(RepeatableBody())
	r.Use(AccessLog())
	// XSS 必须在 AccessLog 之后、且在任何读 gin 参数的一环之前：
	// 日志要记原始报文（排查取证看的是攻击者到底发了什么），而 gin 的
	// c.Query 一旦缓存过 URL.Query()，XSS 再改 RawQuery 就静默失效了。
	r.Use(XSS())
	// I18n 必须在鉴权之前：鉴权失败的提示文案要走词条，得先有语言。
	// 阶段 1 的 Auth 读 clientid，排在 XSS 之后拿到的是清洗后的值。
	r.Use(I18n())
	// Auth 挂在最后：它是唯一可能因为「你是谁」而终止请求的一环，
	// 前面几环（清洗、日志、加解密）对未登录请求同样要生效 ——
	// 否则一次攻击尝试连日志都不会留下。
	r.Use(Auth())
}
