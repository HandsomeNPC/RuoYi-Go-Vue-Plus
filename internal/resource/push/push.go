package push

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/constant"
)

// kickedMessage 同 token 新连接顶替旧连接时发给旧连接的通知。
// 前端按裸串精确比对（utils/push.ts），不能改成 JSON。
const kickedMessage = "kicked"

// manager 推送会话管理器：本进程的连接簿 + Redis 订阅分发 + 心跳巡检。
//
// 会话簿以 token 为唯一键（sa-token 的 token 是每次登录的 UUID，全局唯一），
// userID 记在连接上，故同一用户多终端天然共存。
type manager struct {
	cfg PushConfig

	mu    sync.RWMutex
	conns map[string]*sseConn

	stop     chan struct{}
	stopOnce sync.Once
}

// 包级单例。写只在启动期 Init 一次、读是每条推送的热路径，
// 故用 atomic.Pointer 而非 RWMutex：读是无锁原子 load；nil 表示未启用。
var current atomic.Pointer[manager]

// Init 构建包级推送管理器并订阅 Redis 消息主题。
// 依赖 redis，须在 redis.Init() 之后调用。
//
// 只有注册了推送路由的进程才需要调用——发布端(Publish)无状态，不依赖本函数。
func Init(cfg PushConfig) {
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		panic(fmt.Errorf("push: %w", err))
	}
	if !cfg.Enabled {
		log.Printf("[push] 消息推送已关闭")
		return
	}

	m := &manager{
		cfg:   cfg,
		conns: make(map[string]*sseConn),
		stop:  make(chan struct{}),
	}
	go m.sweepLoop()
	go m.subscribe()

	if old := current.Swap(m); old != nil {
		old.shutdown()
	}
	log.Printf("[push] 消息推送已就绪: path=%s", cfg.Path)
}

// Shutdown 停止心跳、关闭全部连接并清空包级实例，供进程退出时调用。
func Shutdown() {
	if m := current.Swap(nil); m != nil {
		m.shutdown()
	}
}

// shutdown 停掉订阅/心跳协程并关闭全部连接。
func (m *manager) shutdown() {
	m.stopOnce.Do(func() { close(m.stop) })
	m.closeAll()
}

// subscribe 订阅 Redis 消息主题并分发到本地连接。
func (m *manager) subscribe() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := redis.Client().Subscribe(ctx, constant.MessageTopic)
	// ctx 取消只管 SUBSCRIBE 的建立，收不到消息流；真正终止循环的是 sub.Close()。
	// 不能只依赖这里的 defer——它在 range 循环退出后才执行，而循环恰恰被卡在
	// sub.Channel() 上不退出。故另起 goroutine 监听 stop 主动关掉订阅。
	go func() {
		<-m.stop
		_ = sub.Close()
	}()

	for msg := range sub.Channel() {
		var dto PushDTO
		if err := jsonx.Unmarshal([]byte(msg.Payload), &dto); err != nil {
			log.Printf("[push] 解析订阅消息失败: %v", err)
			continue
		}
		m.dispatch(dto)
	}
}

// dispatch 把订阅到的消息投给本地连接：指定了用户就筛选投递，否则广播。
func (m *manager) dispatch(dto PushDTO) {
	if dto.Payload == nil {
		return
	}
	body, err := jsonx.Marshal(dto.Payload)
	if err != nil {
		log.Printf("[push] 序列化消息体失败: %v", err)
		return
	}
	frame := []byte(sseData(string(body)))

	// nil 与空切片同义，都表示广播——system 侧广播就是传 nil。
	if len(dto.UserIDs) == 0 {
		m.deliver(frame, nil)
		return
	}
	targets := make(map[int64]struct{}, len(dto.UserIDs))
	for _, id := range dto.UserIDs {
		targets[id] = struct{}{}
	}
	m.deliver(frame, targets)
}

// deliver 把报文投给匹配的连接，发送失败的顺手剔除。
// targets 为 nil 表示投给全部连接。
func (m *manager) deliver(frame []byte, targets map[int64]struct{}) {
	for token, c := range m.snapshot() {
		if targets != nil {
			if _, ok := targets[c.userID]; !ok {
				continue
			}
		}
		if !c.send(frame) {
			m.drop(token, c)
		}
	}
}

// snapshot 取当前连接快照。
//
// 返回副本而非直接遍历原 map：发送与关闭要在锁外做（可能阻塞在网络 IO 上），
// 而持读锁期间别的 goroutine 无法建连/断连。
func (m *manager) snapshot() map[string]*sseConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]*sseConn, len(m.conns))
	for token, c := range m.conns {
		out[token] = c
	}
	return out
}

// connect 登记一条新连接，并关闭同 token 的旧连接。
//
// 同 token 视为同一终端重连，旧连接必须让位——否则刷新页面会让废弃连接
// 一直留在簿子里，广播时白发一份。
// 顶替通知必须预先裹成 SSE 帧：裸 "kicked" 会被 EventSource 当未知字段丢弃，
// 前端感知不到被顶替，会把接管误判成断线而重连，陷入"重连→再被顶替"的循环。
func (m *manager) connect(token string, c *sseConn) {
	m.mu.Lock()
	old := m.conns[token]
	m.conns[token] = c
	m.mu.Unlock()

	// 关闭旧连接放在锁外做：close 可能阻塞在网络 IO 上，持锁会卡住所有推送。
	if old != nil {
		old.send([]byte(sseData(kickedMessage)))
		old.close()
		log.Printf("[push] 同 token 重连，旧连接已关闭 userId=%d", old.userID)
	}
}

// disconnect 摘掉指定 token 的连接并返回它，供调用方关闭。
func (m *manager) disconnect(token string) *sseConn {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.conns[token]
	delete(m.conns, token)
	return c
}

// drop 剔除一条失效连接并关闭它。
//
// 只在簿子里仍是同一个对象时才摘除：期间可能已被同 token 的新连接顶替，
// 那时按 token 删会把刚建好的连接误删。页面刷新时这个竞态是常规路径。
func (m *manager) drop(token string, expect *sseConn) {
	m.mu.Lock()
	if m.conns[token] == expect {
		delete(m.conns, token)
	}
	m.mu.Unlock()
	expect.close()
}

// closeAll 关闭全部连接，供进程退出时调用。
func (m *manager) closeAll() {
	m.mu.Lock()
	all := m.conns
	m.conns = make(map[string]*sseConn)
	m.mu.Unlock()

	for _, c := range all {
		c.close()
	}
}

// sweepLoop 周期性发心跳并剔除失效连接，随 stop 关闭而退出。
func (m *manager) sweepLoop() {
	ticker := time.NewTicker(m.cfg.heartbeat())
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			for token, c := range m.snapshot() {
				if !c.ping() {
					m.drop(token, c)
				}
			}
		}
	}
}
