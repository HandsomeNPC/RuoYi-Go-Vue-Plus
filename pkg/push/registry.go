package push

import (
	"log"
	"sync"
	"time"
)

// sender 一条已建立的推送连接。sse 与 websocket 各自实现，
// 会话簿只关心「能发一条消息」和「能关掉」，不关心底层协议。
type sender interface {
	// send 投递一条已序列化好的消息。返回 false 表示该连接不可用，应被剔除。
	//
	// 实现必须是非阻塞的：广播会串行遍历同一用户的所有连接，
	// 一个卡住的客户端不能拖住其余连接。
	send(payload []byte) bool
	// ping 发心跳。返回 false 表示连接已失效。
	ping() bool
	// close 关闭连接，重复调用须安全。
	close()
}

// registry 会话簿：userID -> (token -> 连接)，支持同一用户多终端同时在线。
type registry struct {
	mu       sync.RWMutex
	sessions map[int64]map[string]sender
}

func newRegistry() *registry {
	return &registry{sessions: make(map[int64]map[string]sender)}
}

// connect 登记一条新连接，并返回同 token 的旧连接（若有）供调用方善后。
//
// 同 token 视为同一终端重连，旧连接必须让位——否则刷新页面会让废弃连接
// 一直留在簿子里，广播时白发一份：remove-then-put 防止旧连接残留。
// 关闭旧连接放在锁外做：close 可能阻塞在网络 IO 上，持锁会卡住所有推送。
func (r *registry) connect(userID int64, token string, s sender) (old sender) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conns, ok := r.sessions[userID]
	if !ok {
		conns = make(map[string]sender)
		r.sessions[userID] = conns
	}
	old = conns[token]
	conns[token] = s
	return old
}

// disconnect 摘掉指定连接并返回它，供调用方关闭。
// 该用户已无连接时删掉整个 map 项：空 map 留着会随在线用户数只增不减。
func (r *registry) disconnect(userID int64, token string) sender {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.removeLocked(userID, token)
}

// removeLocked 摘除连接，调用方须持写锁。
func (r *registry) removeLocked(userID int64, token string) sender {
	conns, ok := r.sessions[userID]
	if !ok {
		return nil
	}
	s := conns[token]
	delete(conns, token)
	if len(conns) == 0 {
		delete(r.sessions, userID)
	}
	return s
}

// snapshot 取某用户当前的连接快照（token → 连接）。
//
// 返回副本而非直接遍历原 map：发送要在锁外做（可能阻塞），
// 而持读锁期间别的 goroutine 无法 connect/disconnect。
func (r *registry) snapshot(userID int64) map[string]sender {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conns := r.sessions[userID]
	if len(conns) == 0 {
		return nil
	}
	out := make(map[string]sender, len(conns))
	for token, s := range conns {
		out[token] = s
	}
	return out
}

// userIDs 取当前在线用户ID快照，供广播遍历。
func (r *registry) userIDs() []int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]int64, 0, len(r.sessions))
	for id := range r.sessions {
		ids = append(ids, id)
	}
	return ids
}

// sendToUser 把消息投给该用户的全部连接，发送失败的顺手剔除。
func (r *registry) sendToUser(userID int64, payload []byte) {
	for token, s := range r.snapshot(userID) {
		if s.send(payload) {
			continue
		}
		r.drop(userID, token, s)
	}
}

// broadcast 把消息投给本进程全部在线连接。
func (r *registry) broadcast(payload []byte) {
	for _, userID := range r.userIDs() {
		r.sendToUser(userID, payload)
	}
}

// drop 剔除一条失效连接并关闭它。
//
// 只在簿子里仍是同一个对象时才摘除：期间可能已被同 token 的新连接顶替，
// 那时按 token 删会把刚建好的连接误删。
func (r *registry) drop(userID int64, token string, expect sender) {
	r.mu.Lock()
	if conns, ok := r.sessions[userID]; ok && conns[token] == expect {
		r.removeLocked(userID, token)
	}
	r.mu.Unlock()
	expect.close()
}

// monitor 周期性发心跳并剔除失效连接。
// 随 stop 关闭而退出。
func (r *registry) monitor(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			r.sweep()
		}
	}
}

// sweep 一轮心跳巡检。
func (r *registry) sweep() {
	for _, userID := range r.userIDs() {
		for token, s := range r.snapshot(userID) {
			if s.ping() {
				continue
			}
			r.drop(userID, token, s)
		}
	}
}

// closeAll 关闭全部连接，供进程退出时调用。
func (r *registry) closeAll() {
	r.mu.Lock()
	all := r.sessions
	r.sessions = make(map[int64]map[string]sender)
	r.mu.Unlock()

	for _, conns := range all {
		for _, s := range conns {
			s.close()
		}
	}
}

// online 当前在线连接数，供监控与测试用。
func (r *registry) online() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n := 0
	for _, conns := range r.sessions {
		n += len(conns)
	}
	return n
}

// logKickedOld 通知并关闭被同 token 新连接顶替的旧连接。
//
// payload 必须由调用方按 transport 预先裹好（WebSocket 裸文本 / SSE data 帧），
// 本函数只负责投递——SSE 下裸 "kicked" 会被 EventSource 当作未知字段丢弃，
// 前端感知不到被顶替，会把接管误判成断线而重连，陷入"重连→再被顶替"的循环。
func logKickedOld(userID int64, old sender, payload []byte) {
	if old == nil {
		return
	}
	old.send(payload)
	old.close()
	log.Printf("[push] 同 token 重连，旧连接已关闭 userId=%d", userID)
}
