package push

import (
	"log"
	"net/http"
	"sync"
)

// sseConn 一条 SSE 连接。
//
// SSE 是单向流：服务端只能往 http.ResponseWriter 写，且必须 Flush 才会真正
// 落到客户端。gin 的 ResponseWriter 并非并发安全，故与 WebSocket 同构地
// 用「单写者 + 缓冲队列」串行化——但写者不是独立 goroutine，而是持有请求的
// handler 本身（SSE 的响应写入必须在 handler 返回前完成，handler 一返回
// net/http 就会关掉底层连接）。
type sseConn struct {
	w       http.ResponseWriter
	flusher http.Flusher
	out     chan []byte
	done    chan struct{}
	once    sync.Once
}

// newSSEConn 建立 SSE 连接包装。返回 nil 表示 ResponseWriter 不支持流式输出。
func newSSEConn(w http.ResponseWriter, buffer int) *sseConn {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil
	}
	return &sseConn{
		w:       w,
		flusher: flusher,
		out:     make(chan []byte, buffer),
		done:    make(chan struct{}),
	}
}

// send 入队一条消息，队列已满即判定为慢客户端。
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
		log.Printf("[push] SSE 发送队列已满，断开慢连接")
		return false
	}
}

// ping 入队一条注释行心跳（对齐 Java SseEmitter.event().comment("heartbeat")）。
//
// 注释行以 ":" 开头，EventSource 规范要求客户端静默忽略，故它只用于保活、
// 不会触发前端 onmessage。
func (c *sseConn) ping() bool {
	return c.send([]byte(sseComment("heartbeat")))
}

// close 关闭连接，可重复调用。
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
