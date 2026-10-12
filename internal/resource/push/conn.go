package push

import (
	"log"
	"net/http"
	"strings"
	"sync"
)

// sseConn 一条 SSE 连接。
//
// SSE 是单向流：服务端只能往 http.ResponseWriter 写，且必须 Flush 才会真正
// 落到客户端。gin 的 ResponseWriter 并非并发安全，故用「单写者 + 缓冲队列」
// 串行化——但写者不是独立 goroutine，而是持有请求的 handler 本身（SSE 的
// 响应写入必须在 handler 返回前完成，handler 一返回 net/http 就会关掉底层连接）。
type sseConn struct {
	userID  int64
	w       http.ResponseWriter
	flusher http.Flusher
	out     chan []byte
	done    chan struct{}
	once    sync.Once
}

// newSSEConn 建立 SSE 连接包装。返回 nil 表示 ResponseWriter 不支持流式输出。
func newSSEConn(userID int64, w http.ResponseWriter, buffer int) *sseConn {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil
	}
	return &sseConn{
		userID:  userID,
		w:       w,
		flusher: flusher,
		out:     make(chan []byte, buffer),
		done:    make(chan struct{}),
	}
}

// send 入队一条消息，队列已满即判定为慢客户端。
// 非阻塞是契约：广播会串行遍历所有连接，一个卡住的客户端不能拖住其余连接。
func (c *sseConn) send(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.out <- payload:
		return true
	default:
		// 不阻塞等待：推送是可丢的通知，一个卡死的客户端不值得拖住广播。
		log.Printf("[push] 发送队列已满，断开慢连接 userId=%d", c.userID)
		return false
	}
}

// ping 入队一条注释行心跳。
//
// 注释行以 ":" 开头，EventSource 规范要求客户端静默忽略，故它只用于保活、
// 不会触发前端 onmessage。
func (c *sseConn) ping() bool {
	return c.send([]byte(sseComment("heartbeat")))
}

// close 关闭连接，可重复调用。
// 幂等是必需的：registry.drop 与 handler 的 defer 都会关，二次 close(done) 会 panic。
func (c *sseConn) close() {
	c.once.Do(func() { close(c.done) })
}

// closed 返回连接关闭信号，供 handler 的 pump 循环感知。
func (c *sseConn) closed() <-chan struct{} {
	return c.done
}

// writeRaw 直接写一段已成形的 SSE 报文并 Flush。
// 只允许 handler 的 pump 循环调用，它是本连接唯一的写入者。
func (c *sseConn) writeRaw(b []byte) error {
	if _, err := c.w.Write(b); err != nil {
		return err
	}
	c.flusher.Flush()
	return nil
}

// sseData 组装一条 SSE data 事件。
//
// 消息体里的换行必须逐行加 "data: " 前缀：SSE 以空行分隔事件，
// 裸换行会把一条消息截成两个残缺事件。JSON 序列化结果通常无换行，
// 但消息体是业务可控内容，不能假定。
func sseData(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		b.WriteString("data: ")
		b.WriteString(strings.TrimSuffix(line, "\r"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// sseComment 组装一条 SSE 注释行，仅用于保活，客户端会静默忽略。
func sseComment(s string) string {
	return ": " + s + "\n\n"
}
