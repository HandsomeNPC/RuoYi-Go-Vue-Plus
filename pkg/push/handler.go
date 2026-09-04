package push

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// upgrader WebSocket 握手升级器。
//
// CheckOrigin 恒放行：跨域策略统一由 pkg/middleware 的 CORS 中间件决定。
// 注意 CORS 中间件管不到 WebSocket 握手——浏览器对 ws:// 不发预检，
// 故这里必须自己表态，否则 gorilla 默认只接受同源请求，前端本地开发
// （vite 端口与后端不同源）会被直接拒掉。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// Handler 返回推送连接端点，按配置的 transport 决定走 SSE 还是 WebSocket。
//
// 必须注册在带登录校验的路由组内：会话按 userID + token 归档，
// 拿不到登录态就没法投递，须在握手阶段取登录态。
func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := Default()
		if m == nil {
			_ = c.Error(errs.New(response.CodeFail, "消息推送未启用", ""))
			return
		}
		if m.cfg.IsWebSocket() {
			m.serveWebSocket(c)
			return
		}
		m.serveSSE(c)
	}
}

// CloseHandler 返回主动断开当前连接的端点。
func CloseHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := Default()
		if m == nil {
			c.JSON(http.StatusOK, response.OkVoid())
			return
		}
		userID, token, ok := identify(c)
		if ok {
			if s := m.registry.disconnect(userID, token); s != nil {
				s.close()
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

// serveWebSocket 处理 WebSocket 连接的完整生命周期。
func (m *Manager) serveWebSocket(c *gin.Context) {
	userID, token, ok := identify(c)
	if !ok {
		// 未登录不升级：升级后再关连接前端只看到一个立刻断开的 socket，
		// 拿不到原因。这里还没握手，比起 CloseStatus.BAD_DATA 可以更直接地回 401。
		_ = c.Error(errs.New(response.CodeUnauthorized, "未登录，无法建立推送连接", ""))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时它已自行写过响应，不能再走 c.Error 二次渲染。
		log.Printf("[push] WebSocket 握手失败 userId=%d: %v", userID, err)
		return
	}

	wc := newWSConn(conn, m.cfg.SendBuffer)
	logKickedOld(userID, m.registry.connect(userID, token, wc),
		m.frame([]byte(kickedMessage)))
	log.Printf("[push] WebSocket 已连接 userId=%d", userID)

	defer func() {
		m.registry.drop(userID, token, wc)
		log.Printf("[push] WebSocket 已断开 userId=%d", userID)
	}()

	// 收到 pong 即视为链路存活；不设读超时，靠 monitor 的 ping 探活。
	conn.SetPongHandler(func(string) error { return nil })

	// 读循环兼作连接存活的判据：ReadMessage 返回错误即对端已走。
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		m.handleClientMessage(wc, msg)
	}
}

// handleClientMessage 处理客户端上行消息。
//
// 目前只认心跳。原实现会把其它文本当自定义消息回发给该用户，
// 这里有意不做：那条路径等于允许前端借服务端广播任意内容，而本项目没有任何
// 业务用到它，留着只是多一个可被滥用的入口。真需要时在此处补即可。
func (m *Manager) handleClientMessage(wc *wsConn, msg []byte) {
	if !isPing(msg) {
		return
	}
	wc.send([]byte(pongMessage))
}

// isPing 判定是否心跳请求。
//
// 同时认裸字符串 "ping" 与 {"type":"ping"} 两种形态：原实现比对的是裸串，
// 而仓库自带前端（utils/message.ts）发的是 JSON。只认一种会让另一方的
// pongTimeout 超时，进而误判断线并反复重连。
func isPing(msg []byte) bool {
	s := strings.TrimSpace(string(msg))
	if strings.EqualFold(s, pingMessage) {
		return true
	}
	return strings.Contains(s, `"`+pingMessage+`"`)
}

// serveSSE 处理 SSE 连接：握手后在 handler 内阻塞泵消息直至断开。
//
// 与 WebSocket 的关键差异：SSE 的写入必须在 handler 返回前完成——handler
// 一返回，net/http 就会结束响应体并关闭连接。故这里不能"注册完就返回"，
// 必须自己守在循环里。
func (m *Manager) serveSSE(c *gin.Context) {
	userID, token, ok := identify(c)
	if !ok {
		_ = c.Error(errs.New(response.CodeUnauthorized, "未登录，无法建立推送连接", ""))
		return
	}

	sc := newSSEConn(c.Writer, m.cfg.SendBuffer)
	if sc == nil {
		_ = c.Error(errs.New(response.CodeFail, "当前服务不支持 SSE 流式响应", ""))
		return
	}

	prepareSSEHeaders(c)
	logKickedOld(userID, m.registry.connect(userID, token, sc),
		m.frame([]byte(kickedMessage)))
	log.Printf("[push] SSE 已连接 userId=%d", userID)

	defer func() {
		m.registry.drop(userID, token, sc)
		log.Printf("[push] SSE 已断开 userId=%d", userID)
	}()

	// 先回一条注释确认连接建立：
	// 前端据此知道流已就绪，且能立刻探明代理有没有缓冲住响应。
	if err := sc.writeRaw([]byte(sseComment("connected"))); err != nil {
		return
	}

	// 超时上限到点主动收流，由前端 EventSource 自动重连。
	// 不留常驻不死的连接，避免服务端连接数只增不减。
	timeout := time.NewTimer(m.cfg.SSETimeout())
	defer timeout.Stop()

	for {
		select {
		case <-c.Request.Context().Done(): // 客户端断开
			return
		case <-sc.closed(): // 被顶替或被判定为慢连接
			return
		case <-timeout.C:
			return
		case payload := <-sc.out:
			if err := sc.writeRaw(payload); err != nil {
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
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
}
