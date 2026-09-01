package push

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// writeWait 单条消息的写超时。到点即认定连接不可用并剔除，
// 对应 Java MessageProperties.webSocketSendTimeLimit。
const writeWait = 10 * time.Second

// wsConn 一条 WebSocket 连接。
//
// gorilla/websocket 明确规定同一连接上并发写会 panic，而广播是多 goroutine
// 触发的。Java 侧靠 ConcurrentWebSocketSessionDecorator 加锁串行化，
// 这里改用「单写者 goroutine + 有缓冲 channel」——Go 里更地道，且能顺带
// 用缓冲长度界定慢客户端：写满即放弃该连接，不让它堵住广播。
type wsConn struct {
	conn *websocket.Conn
	// out 待发消息队列，由 writePump 独占消费。
	out chan []byte
	// done 关闭信号，兼作 close 的幂等开关。
	done chan struct{}
	once sync.Once
}

// newWSConn 建立连接包装并启动写协程。
func newWSConn(conn *websocket.Conn, buffer int) *wsConn {
	c := &wsConn{
		conn: conn,
		out:  make(chan []byte, buffer),
		done: make(chan struct{}),
	}
	go c.writePump()
	return c
}

// writePump 唯一的写入者，串行消费 out 队列。
func (c *wsConn) writePump() {
	for {
		select {
		case <-c.done:
			return
		case payload, ok := <-c.out:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				// 写失败意味着连接已废，唤醒读协程收摊。
				c.close()
				return
			}
		}
	}
}

// send 入队一条消息。队列已满即判定为慢客户端，返回 false 由会话簿剔除。
func (c *wsConn) send(payload []byte) bool {
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
		log.Printf("[push] 发送队列已满，断开慢连接")
		return false
	}
}

// ping 发 WebSocket 控制帧心跳。
//
// 走 ping 帧而不走 out 队列：控制帧由 gorilla 内部加锁保护，可与 writePump
// 并发调用（这是 WriteControl 与 WriteMessage 的明确差异），因此队列积压时
// 心跳依然能探到真实链路状态。
func (c *wsConn) ping() bool {
	select {
	case <-c.done:
		return false
	default:
	}
	err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
	return err == nil
}

// close 关闭连接，可重复调用。
func (c *wsConn) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
