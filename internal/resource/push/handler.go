package push

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
)

// bearerPrefix query token 里可能带的授权前缀（大小写不敏感）。
const bearerPrefix = "bearer "

// Handler 返回 SSE 推送连接端点。
//
// 必须注册在带登录校验的路由组内：会话按 token 归档、按 userID 投递，
// 拿不到登录态就没法投递，须在握手阶段取登录态。
func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := current.Load()
		if m == nil {
			// 未启用时路由本就不注册，正常到不了这里。
			_ = c.Error(errs.New(response.CodeFail, "消息推送未启用", ""))
			return
		}
		m.serveSSE(c)
	}
}

// CloseHandler 返回主动断开当前连接的端点。
func CloseHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m := current.Load(); m != nil {
			if _, token, ok := identify(c); ok {
				if conn := m.disconnect(token); conn != nil {
					conn.close()
				}
			}
		}
		c.JSON(http.StatusOK, response.OkVoid())
	}
}

// identify 取当前请求的用户ID与 token，二者缺一即视为未登录。
func identify(c *gin.Context) (userID int64, token string, ok bool) {
	userID = loginhelper.GetUserID(c)
	token = sagin.GetTokenFromCtx(c)
	return userID, token, userID != 0 && token != ""
}

// serveSSE 处理 SSE 连接：握手后在 handler 内阻塞泵消息直至断开。
//
// SSE 的写入必须在 handler 返回前完成——handler 一返回，net/http 就会结束
// 响应体并关闭连接。故这里不能"注册完就返回"，必须自己守在循环里。
func (m *manager) serveSSE(c *gin.Context) {
	userID, token, ok := identify(c)
	if !ok {
		_ = c.Error(errs.New(response.CodeUnauthorized, "未登录，无法建立推送连接", ""))
		return
	}

	conn := newSSEConn(userID, c.Writer, m.cfg.SendBuffer)
	if conn == nil {
		// 不 Flush 则消息会攒在缓冲里——「连上了但收不到」比 500 难排查得多。
		_ = c.Error(errs.New(response.CodeFail, "当前服务不支持 SSE 流式响应", ""))
		return
	}

	prepareSSEHeaders(c)
	m.connect(token, conn)
	log.Printf("[push] SSE 已连接 userId=%d", userID)

	defer func() {
		m.drop(token, conn)
		log.Printf("[push] SSE 已断开 userId=%d", userID)
	}()

	// 先回一条注释确认连接建立：
	// 前端据此知道流已就绪，且能立刻探明代理有没有缓冲住响应。
	if err := conn.writeRaw([]byte(sseComment("connected"))); err != nil {
		return
	}

	// 超时上限到点主动收流，由前端 EventSource 自动重连。
	// 不留常驻不死的连接，避免服务端连接数只增不减。
	timeout := time.NewTimer(m.cfg.sseTimeout())
	defer timeout.Stop()

	for {
		select {
		case <-c.Request.Context().Done(): // 客户端断开
			return
		case <-conn.closed(): // 被顶替或被判定为慢连接
			return
		case <-timeout.C:
			return
		case payload := <-conn.out:
			if err := conn.writeRaw(payload); err != nil {
				return
			}
		}
	}
}

// prepareSSEHeaders 设置 SSE 响应头。
func prepareSSEHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// 关掉 nginx 的响应缓冲，否则消息会被攒在代理里直到缓冲写满才下发。
	h.Set("X-Accel-Buffering", "no")
	// 先发头再 Flush：这是前端 onopen 及时触发的前提。
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
}

// NormalizeQueryToken 把 query 里 `?Authorization=Bearer xxx` 的前缀剥掉，
// 使其与 header 形态一致。须注册在 TokenInterceptor 之前。
//
// 为什么必须有这一层：SSE 的 EventSource 不能自定义请求头，前端只能把 token
// 塞进 query（见 utils/push.ts）。而 sa-token-go 的 ReadTokenFromRequest 只在
// header 分支调 extractBearerToken，query 分支仅做 CutTokenPrefix —— 后者在
// 未配置 TokenPrefix 时原样返回，于是拿着 "Bearer eyJ..." 整串去查会话，
// 必然查不到而 401。
//
// 改全局 TokenPrefix 也能让 CutTokenPrefix 生效，但那会连带要求所有普通接口的
// header 都带前缀，牵动面太大；只在推送这条链路上做规范化更克制。
func NormalizeQueryToken(tokenName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Request.URL.RawQuery
		// 快速排除：绝大多数请求不带 query token，避免逐个请求解析 query。
		// 只按 "bearer" 判断而不含空格——RawQuery 里空格是 %20 或 +，
		// 拿 "bearer " 去匹配编码后的串永远不命中。
		if raw == "" || !strings.Contains(strings.ToLower(raw), "bearer") {
			c.Next()
			return
		}

		values, err := url.ParseQuery(raw)
		if err != nil {
			c.Next()
			return
		}
		got := values.Get(tokenName)
		if len(got) <= len(bearerPrefix) ||
			!strings.EqualFold(got[:len(bearerPrefix)], bearerPrefix) {
			c.Next()
			return
		}

		values.Set(tokenName, strings.TrimSpace(got[len(bearerPrefix):]))
		c.Request.URL.RawQuery = values.Encode()
		// 不必清 gin 的 query 缓存：它是懒构建的（initQueryCache 在首次
		// c.Query 时才从 RawQuery 取值），而本中间件跑在 TokenInterceptor
		// 之前，此刻还没有人读过 query。但正因如此，本中间件必须排在所有
		// 读 query 的中间件之前，否则改动对已缓存的值无效。

		c.Next()
	}
}
