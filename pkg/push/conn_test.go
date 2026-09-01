package push

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialTestWS 起一个真实 WebSocket 服务并连上，返回客户端连接与服务端包装。
// 与 registry_test.go 的 fakeConn 互补：这里验的是 wsConn 与 gorilla 的真实交互。
func dialTestWS(t *testing.T, buffer int) (*websocket.Conn, *wsConn) {
	t.Helper()

	serverConn := make(chan *wsConn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("服务端升级失败: %v", err)
			return
		}
		wc := newWSConn(c, buffer)
		serverConn <- wc
		// 读循环维持连接：不读的话对端的关闭帧无人处理。
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	client, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("客户端连接失败: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case wc := <-serverConn:
		return client, wc
	case <-time.After(3 * time.Second):
		t.Fatal("等待服务端连接超时")
		return nil, nil
	}
}

// TestWSConnSendDelivers 消息能真正投到客户端。
func TestWSConnSendDelivers(t *testing.T) {
	client, wc := dialTestWS(t, 8)

	if !wc.send([]byte(`{"message":"hi"}`)) {
		t.Fatal("send 应成功入队")
	}

	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	typ, data, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("客户端读取失败: %v", err)
	}
	if typ != websocket.TextMessage {
		t.Errorf("消息类型 = %d, 期望 TextMessage(%d)", typ, websocket.TextMessage)
	}
	if got, want := string(data), `{"message":"hi"}`; got != want {
		t.Errorf("收到 %q, 期望 %q", got, want)
	}
}

// TestWSConnSendAfterCloseFails 已关闭的连接拒绝入队，由会话簿据此剔除。
func TestWSConnSendAfterCloseFails(t *testing.T) {
	_, wc := dialTestWS(t, 8)
	wc.close()

	if wc.send([]byte("hi")) {
		t.Error("已关闭连接的 send 应返回 false")
	}
}

// TestWSConnCloseIsIdempotent 重复 close 不 panic。
//
// 会话簿的 drop 与读循环的 defer 都会调 close，双重关闭是常态而非异常；
// 没有 sync.Once 兜着的话第二次 close(c.done) 会直接 panic。
func TestWSConnCloseIsIdempotent(t *testing.T) {
	_, wc := dialTestWS(t, 8)
	wc.close()
	wc.close()
	wc.close()
}

// TestWSConnFullBufferRejects 缓冲写满即拒绝，不阻塞调用方。
//
// 这是慢客户端的兜底：send 必须立刻返回 false 让会话簿剔除它，
// 而不是挂在 channel 上把广播协程一起拖住。
func TestWSConnFullBufferRejects(t *testing.T) {
	// 缓冲设 1 并让写协程停在第一条上，后续必然堆积。
	_, wc := dialTestWS(t, 1)
	// 抢占 writePump：先关掉底层连接，写就会失败并停摆。
	_ = wc.conn.Close()

	// 反复投递直到被拒——writePump 卡住后缓冲很快填满。
	rejected := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !wc.send([]byte("x")) {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Error("缓冲写满后 send 应返回 false 而非无限入队")
	}
}

// TestWSConnPingReachesClient 心跳帧能触达客户端的 ping handler。
//
// 走 WriteControl 而非 out 队列：控制帧由 gorilla 内部加锁，可与 writePump
// 并发调用，故队列积压时心跳仍能探到真实链路状态。
func TestWSConnPingReachesClient(t *testing.T) {
	client, wc := dialTestWS(t, 8)

	got := make(chan struct{}, 1)
	client.SetPingHandler(func(string) error {
		select {
		case got <- struct{}{}:
		default:
		}
		return nil
	})
	// 客户端只有在读取时才会处理控制帧。
	go func() {
		_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, _ = client.ReadMessage()
	}()

	if !wc.ping() {
		t.Fatal("ping 应成功")
	}
	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Error("客户端未收到 ping 帧")
	}
}

// TestWSConnPingFailsAfterClose 连接关闭后 ping 失败，供巡检剔除。
func TestWSConnPingFailsAfterClose(t *testing.T) {
	_, wc := dialTestWS(t, 8)
	wc.close()

	if wc.ping() {
		t.Error("已关闭连接的 ping 应返回 false")
	}
}

// TestNewSSEConnRejectsNonFlusher 不支持 Flush 的 ResponseWriter 直接拒绝。
//
// SSE 必须能逐条 Flush，否则消息会攒在缓冲里——那种"连上了但收不到"
// 比直接报错难查得多，故在建连时就拦掉。
func TestNewSSEConnRejectsNonFlusher(t *testing.T) {
	if got := newSSEConn(nonFlusher{}, 8); got != nil {
		t.Error("不支持 Flusher 的 writer 应返回 nil")
	}
}

// nonFlusher 只实现 http.ResponseWriter，不实现 http.Flusher。
type nonFlusher struct{}

func (nonFlusher) Header() http.Header       { return http.Header{} }
func (nonFlusher) Write([]byte) (int, error) { return 0, nil }
func (nonFlusher) WriteHeader(int)           {}

// TestSSEConnSendAndClose SSE 连接的入队、关闭与幂等关闭。
func TestSSEConnSendAndClose(t *testing.T) {
	rec := httptest.NewRecorder() // httptest.ResponseRecorder 实现了 Flusher
	sc := newSSEConn(rec, 2)
	if sc == nil {
		t.Fatal("应成功构造 SSE 连接")
	}

	if !sc.send([]byte("data: a\n\n")) {
		t.Error("首条消息应入队成功")
	}
	sc.close()
	sc.close() // 幂等，不该 panic

	if sc.send([]byte("data: b\n\n")) {
		t.Error("已关闭连接的 send 应返回 false")
	}
	select {
	case <-sc.closed():
	default:
		t.Error("closed() 应已就绪")
	}
}

// TestSSEConnFullBufferRejects SSE 缓冲写满同样拒绝而不阻塞。
func TestSSEConnFullBufferRejects(t *testing.T) {
	sc := newSSEConn(httptest.NewRecorder(), 1)
	if !sc.send([]byte("a")) {
		t.Fatal("首条应入队成功")
	}
	// 无人消费 out，第二条就该被拒。
	if sc.send([]byte("b")) {
		t.Error("缓冲写满后应返回 false")
	}
}

// TestSSEConnPingEnqueuesComment SSE 心跳走注释行，客户端静默忽略。
func TestSSEConnPingEnqueuesComment(t *testing.T) {
	sc := newSSEConn(httptest.NewRecorder(), 4)
	if !sc.ping() {
		t.Fatal("ping 应入队成功")
	}
	got := string(<-sc.out)
	if want := sseComment("heartbeat"); got != want {
		t.Errorf("心跳内容 = %q, 期望 %q", got, want)
	}
}
