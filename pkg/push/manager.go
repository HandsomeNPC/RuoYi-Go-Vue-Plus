package push

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/jsonx"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
)

// 客户端/服务端约定的控制消息。
const (
	// pingMessage 客户端心跳请求。
	pingMessage = "ping"
	// pongMessage 服务端心跳响应。
	pongMessage = "pong"
	// kickedMessage 同 token 新连接顶替旧连接时发给旧连接的通知。
	kickedMessage = "kicked"
)

// Manager 推送会话管理器。
type Manager struct {
	cfg      config.PushConfig
	registry *registry
	stop     chan struct{}
	stopOnce sync.Once
}

// 包级默认实例。
var (
	mu             sync.RWMutex
	defaultManager *Manager
)

// Init 按配置构建包级推送管理器并订阅 Redis 消息主题。
// 依赖 pkg/redis，须在 redis.Init() 之后调用。
func Init() {
	c := config.Get()
	if !c.Push.Enabled {
		log.Printf("[%s] 消息推送已关闭", c.Server.Name)
		return
	}

	m := &Manager{
		cfg:      c.Push,
		registry: newRegistry(),
		stop:     make(chan struct{}),
	}

	go m.registry.monitor(c.Push.Heartbeat(), m.stop)
	go m.subscribe()

	mu.Lock()
	defaultManager = m
	mu.Unlock()

	log.Printf("[%s] 消息推送已就绪: transport=%s path=%s",
		c.Server.Name, c.Push.Transport, c.Push.Path)
}

// Default 返回包级实例，未启用推送时返回 nil。
// 调用方须判空——推送是可选能力，关掉它不该让业务路径崩掉。
func Default() *Manager {
	mu.RLock()
	defer mu.RUnlock()
	return defaultManager
}

// Shutdown 停止心跳、关闭全部连接并清空包级实例，供进程退出时调用。
func Shutdown() {
	mu.Lock()
	m := defaultManager
	defaultManager = nil
	mu.Unlock()

	if m == nil {
		return
	}
	m.stopOnce.Do(func() { close(m.stop) })
	m.registry.closeAll()
}

// Config 返回本管理器的推送配置。
func (m *Manager) Config() config.PushConfig {
	return m.cfg
}

// Online 当前本进程在线连接数。
func (m *Manager) Online() int {
	return m.registry.online()
}

// Publish 把消息发布到 Redis 主题，由各实例的订阅方投给本地连接。
//
// 必须走 Redis 而不能直接写本地会话：nginx 后可能有多个实例，
// 用户连在哪个实例上不确定，只发本地会话会让其余实例上的用户收不到。
func Publish(ctx context.Context, dto PushDTO) error {
	if dto.Payload == nil {
		return nil
	}
	// 推送未启用时静默跳过。
	if Default() == nil {
		return nil
	}

	payload, err := jsonx.Marshal(dto)
	if err != nil {
		return fmt.Errorf("push: 序列化推送消息失败: %w", err)
	}
	if err := pkgredis.Client().Publish(ctx, constant.MessageTopic, payload).Err(); err != nil {
		return fmt.Errorf("push: 发布推送消息失败: %w", err)
	}
	return nil
}

// PublishAll 广播给全部在线用户。
func PublishAll(ctx context.Context, payload *systemdto.PushPayloadDTO) error {
	return Publish(ctx, Broadcast(payload))
}

// PublishUsers 发布给指定用户。
func PublishUsers(ctx context.Context, userIDs []int64,
	payload *systemdto.PushPayloadDTO) error {

	return Publish(ctx, ToUsers(userIDs, payload))
}

// subscribe 订阅 Redis 消息主题并分发到本地连接。
func (m *Manager) subscribe() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := pkgredis.Client().Subscribe(ctx, constant.MessageTopic)
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

// dispatch 把订阅到的消息投给本地连接：指定了用户就单发，否则广播。
func (m *Manager) dispatch(dto PushDTO) {
	if dto.Payload == nil {
		return
	}
	body, err := jsonx.Marshal(dto.Payload)
	if err != nil {
		log.Printf("[push] 序列化消息体失败: %v", err)
		return
	}

	if len(dto.UserIDs) == 0 {
		m.registry.broadcast(m.frame(body))
		return
	}
	frame := m.frame(body)
	for _, userID := range dto.UserIDs {
		m.registry.sendToUser(userID, frame)
	}
}

// frame 把消息体裹成对应传输方式的报文。
//
// WebSocket 直接发 JSON 文本；SSE 必须包成 "data: ...\n\n" 事件帧，
// 否则前端 EventSource 收不到 onmessage。
func (m *Manager) frame(body []byte) []byte {
	if m.cfg.IsWebSocket() {
		return body
	}
	return []byte(sseData(string(body)))
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
