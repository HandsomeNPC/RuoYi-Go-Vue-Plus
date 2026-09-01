package push

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConn 可控的假连接，用于验证会话簿的行为而不起真实网络。
type fakeConn struct {
	mu     sync.Mutex
	sent   [][]byte
	closed bool
	// full 为真时 send 一律失败，模拟队列写满的慢客户端。
	full bool
	// dead 为真时 ping 失败，模拟已断开但尚未被剔除的连接。
	dead bool
	// block 非 nil 时 send 会阻塞在它上，用于验证广播不被单个连接拖住。
	block chan struct{}
}

func (f *fakeConn) send(payload []byte) bool {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.full || f.closed {
		return false
	}
	f.sent = append(f.sent, payload)
	return true
}

func (f *fakeConn) ping() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.dead && !f.closed
}

func (f *fakeConn) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func (f *fakeConn) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func (f *fakeConn) messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.sent))
	for _, m := range f.sent {
		out = append(out, string(m))
	}
	return out
}

// TestRegistryConnectReplacesSameToken 同 token 重连时旧连接让位。
//
// 不替换的话，刷新页面会让废弃连接一直留在簿子里，广播时白发一份。
func TestRegistryConnectReplacesSameToken(t *testing.T) {
	r := newRegistry()
	old, fresh := &fakeConn{}, &fakeConn{}

	if got := r.connect(1, "tok", old); got != nil {
		t.Fatalf("首次连接不应返回旧连接, got %#v", got)
	}
	got := r.connect(1, "tok", fresh)
	if got != old {
		t.Fatalf("同 token 重连应返回旧连接以便善后, got %#v", got)
	}
	if r.online() != 1 {
		t.Errorf("同 token 替换后应只剩 1 条连接, got %d", r.online())
	}

	// 新连接才该收到消息。
	r.sendToUser(1, []byte("hi"))
	if len(fresh.messages()) != 1 {
		t.Errorf("新连接应收到消息, got %v", fresh.messages())
	}
	if len(old.messages()) != 0 {
		t.Errorf("被替换的旧连接不该再收消息, got %v", old.messages())
	}
}

// TestLogKickedOldNotifiesThenCloses 旧连接先收到 kicked 通知再被关闭。
//
// 前端据此区分「被顶替」与「网络断开」，后者才该触发重连。
func TestLogKickedOldNotifiesThenCloses(t *testing.T) {
	old := &fakeConn{}
	logKickedOld(1, old, []byte(kickedMessage))

	msgs := old.messages()
	if len(msgs) != 1 || msgs[0] != kickedMessage {
		t.Errorf("旧连接应收到 %q, got %v", kickedMessage, msgs)
	}
	if !old.isClosed() {
		t.Error("旧连接应被关闭")
	}
}

// TestLogKickedOldPreservesFrame 顶替通知必须按 transport 裹好再投递。
//
// SSE 下发给旧连接的 "kicked" 得是 "data: kicked\n\n"：裸 "kicked" 会被
// EventSource 当作未知字段丢弃，前端感知不到被顶替，会把接管误判成断线而
// 重连，陷入"重连→再被顶替"的循环。调用方（handler）负责裹帧，
// 本测试钉住"收到的就是裹好的载荷"这一契约。
func TestLogKickedOldPreservesFrame(t *testing.T) {
	old := &fakeConn{}
	logKickedOld(1, old, []byte(sseData(kickedMessage)))

	msgs := old.messages()
	if len(msgs) != 1 || msgs[0] != "data: kicked\n\n" {
		t.Errorf("SSE 下顶替通知应为帧格式, got %v", msgs)
	}
}

// TestRegistryMultipleDevices 同一用户多终端并存，广播时人人有份。
func TestRegistryMultipleDevices(t *testing.T) {
	r := newRegistry()
	pc, app := &fakeConn{}, &fakeConn{}
	r.connect(1, "pc-token", pc)
	r.connect(1, "app-token", app)

	if r.online() != 2 {
		t.Fatalf("应有 2 条连接, got %d", r.online())
	}
	r.sendToUser(1, []byte("hi"))
	if len(pc.messages()) != 1 || len(app.messages()) != 1 {
		t.Errorf("两个终端都该收到, pc=%v app=%v", pc.messages(), app.messages())
	}
}

// TestRegistryDisconnectCleansUpUser 用户最后一条连接摘掉后不留空 map。
//
// 空 map 留着会随历史在线用户数只增不减，是典型的慢性内存泄漏。
func TestRegistryDisconnectCleansUpUser(t *testing.T) {
	r := newRegistry()
	r.connect(1, "a", &fakeConn{})
	r.connect(1, "b", &fakeConn{})

	r.disconnect(1, "a")
	if got := len(r.sessions); got != 1 {
		t.Errorf("还有一条连接时不该删掉用户项, got %d", got)
	}
	r.disconnect(1, "b")
	if got := len(r.sessions); got != 0 {
		t.Errorf("最后一条连接摘掉后应删掉用户项, got %d", got)
	}
}

// TestRegistryDropsSlowClient 发送失败(队列写满)的连接被剔除并关闭。
func TestRegistryDropsSlowClient(t *testing.T) {
	r := newRegistry()
	slow := &fakeConn{full: true}
	r.connect(1, "tok", slow)

	r.sendToUser(1, []byte("hi"))

	if r.online() != 0 {
		t.Errorf("发送失败的连接应被剔除, online=%d", r.online())
	}
	if !slow.isClosed() {
		t.Error("被剔除的连接应被关闭")
	}
}

// TestBroadcastNotBlockedBySlowClient 一个卡死的连接不能拖住其余连接的广播。
//
// 推送是可丢的通知；若广播串行等待每个客户端确认，一个挂起的 TCP 连接
// 就能让全站公告推送停摆。
func TestBroadcastNotBlockedBySlowClient(t *testing.T) {
	r := newRegistry()
	blocked := &fakeConn{block: make(chan struct{})}
	normal := &fakeConn{}
	r.connect(1, "stuck", blocked)
	r.connect(2, "ok", normal)

	done := make(chan struct{})
	go func() {
		r.broadcast([]byte("hi"))
		close(done)
	}()

	// 放开卡住的连接，广播才能收尾；关键是它不该无限期挂死。
	time.AfterFunc(50*time.Millisecond, func() { close(blocked.block) })

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("广播被单个慢连接拖死")
	}
	if len(normal.messages()) != 1 {
		t.Errorf("正常连接应收到广播, got %v", normal.messages())
	}
}

// TestRegistrySweepRemovesDeadConns 心跳巡检剔除已失效连接。
func TestRegistrySweepRemovesDeadConns(t *testing.T) {
	r := newRegistry()
	alive, dead := &fakeConn{}, &fakeConn{dead: true}
	r.connect(1, "alive", alive)
	r.connect(2, "dead", dead)

	r.sweep()

	if r.online() != 1 {
		t.Errorf("巡检后应只剩 1 条存活连接, got %d", r.online())
	}
	if !dead.isClosed() {
		t.Error("失效连接应被关闭")
	}
	if alive.isClosed() {
		t.Error("存活连接不该被误关")
	}
}

// TestRegistryDropKeepsReplacement drop 不能误删已被顶替上来的新连接。
//
// 剔除的判据是"簿子里仍是同一个对象"而非 token：慢连接被判定失效的同时
// 可能已有同 token 新连接建好，按 token 删会把刚建好的连接误删。
func TestRegistryDropKeepsReplacement(t *testing.T) {
	r := newRegistry()
	old, fresh := &fakeConn{}, &fakeConn{}
	r.connect(1, "tok", old)
	r.connect(1, "tok", fresh)

	// 拿已被替换掉的 old 去 drop，不该动到 fresh。
	r.drop(1, "tok", old)

	if r.online() != 1 {
		t.Fatalf("新连接应留在簿子里, online=%d", r.online())
	}
	if fresh.isClosed() {
		t.Error("新连接被误关了")
	}
	r.sendToUser(1, []byte("hi"))
	if len(fresh.messages()) != 1 {
		t.Errorf("新连接应仍能收消息, got %v", fresh.messages())
	}
}

// TestRegistryCloseAll 关闭全部连接并清空簿子。
func TestRegistryCloseAll(t *testing.T) {
	r := newRegistry()
	a, b := &fakeConn{}, &fakeConn{}
	r.connect(1, "a", a)
	r.connect(2, "b", b)

	r.closeAll()

	if r.online() != 0 {
		t.Errorf("closeAll 后应无连接, got %d", r.online())
	}
	if !a.isClosed() || !b.isClosed() {
		t.Error("全部连接都该被关闭")
	}
}

// TestRegistryConcurrentAccess 并发连接/发送/断开不触发 data race。
// 靠 -race 检出，这里只保证不 panic、不死锁。
func TestRegistryConcurrentAccess(t *testing.T) {
	r := newRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(3)
		userID := int64(i % 5)
		go func() { defer wg.Done(); r.connect(userID, "tok", &fakeConn{}) }()
		go func() { defer wg.Done(); r.sendToUser(userID, []byte("hi")) }()
		go func() { defer wg.Done(); r.broadcast([]byte("all")) }()
	}
	wg.Wait()
	r.closeAll()
}

// TestIsPing 心跳同时认裸串与 JSON 两种形态。
//
// Java 侧客户端发裸 "ping"，而仓库自带前端发 {"type":"ping"}。
// 只认一种会让另一方 pongTimeout 超时，进而误判断线并反复重连。
func TestIsPing(t *testing.T) {
	for _, in := range []string{
		"ping",
		"PING",
		" ping ",
		`{"type":"ping"}`,
		`{"type": "ping", "t": 1}`,
	} {
		if !isPing([]byte(in)) {
			t.Errorf("isPing(%q) = false, 应识别为心跳", in)
		}
	}
	for _, in := range []string{"", "pong", "hello", `{"type":"message"}`} {
		if isPing([]byte(in)) {
			t.Errorf("isPing(%q) = true, 不应识别为心跳", in)
		}
	}
}

// TestSSEData SSE 事件帧格式：每行加 data: 前缀，以空行结束。
func TestSSEData(t *testing.T) {
	got := sseData(`{"a":1}`)
	if want := "data: {\"a\":1}\n\n"; got != want {
		t.Errorf("sseData() = %q, 期望 %q", got, want)
	}
}

// TestSSEDataMultiline 消息体含换行时逐行加前缀。
//
// SSE 以空行分隔事件，裸换行会把一条消息截成两个残缺事件——
// 前端表现为收到一半 JSON 而解析失败。
func TestSSEDataMultiline(t *testing.T) {
	got := sseData("line1\nline2")
	want := "data: line1\ndata: line2\n\n"
	if got != want {
		t.Errorf("sseData() = %q, 期望 %q", got, want)
	}
	// 除结尾的事件分隔空行外，中间不能出现空行。
	if strings.Contains(strings.TrimSuffix(got, "\n\n"), "\n\n") {
		t.Errorf("事件体中间出现空行，会截断事件: %q", got)
	}
}

// TestSSEDataStripsCR CRLF 换行不留下裸 \r。
func TestSSEDataStripsCR(t *testing.T) {
	got := sseData("a\r\nb")
	if want := "data: a\ndata: b\n\n"; got != want {
		t.Errorf("sseData() = %q, 期望 %q", got, want)
	}
}

// TestSSEComment 注释行用于保活，客户端静默忽略。
func TestSSEComment(t *testing.T) {
	if got, want := sseComment("heartbeat"), ": heartbeat\n\n"; got != want {
		t.Errorf("sseComment() = %q, 期望 %q", got, want)
	}
}
