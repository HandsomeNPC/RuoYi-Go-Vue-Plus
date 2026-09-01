package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// 消息推送传输方式，对照 Java MessageTransportEnum。
const (
	// PushTransportSSE Server-Sent Events，单向轻量传输。
	PushTransportSSE = "sse"
	// PushTransportWebSocket 全双工长连接，支持客户端回传。
	PushTransportWebSocket = "websocket"
)

// defaultPushPath 默认连接路径，与 Java MessageProperties.path 一致。
//
// 注意：仓库内自带的前端 apps/web-antd/src/utils/message.ts 连的是
// /resource/sse 与 /resource/websocket，与本默认值对不上——Java 侧同样如此，
// 靠改配置对齐。要让自带前端直接连通，把 push.path 改成对应路径即可。
const defaultPushPath = "/resource/message"

// PushConfig 消息推送配置，对应 yaml 的 push 段（对照 Java MessageProperties）。
//
// 未暴露 allowedOrigins：跨域已由 pkg/middleware 的 CORS 统一管，
// 握手期再加一套白名单只会两处打架。
type PushConfig struct {
	// Enabled 是否启用消息推送。关闭时不注册推送路由，且发布消息直接跳过。
	Enabled bool `mapstructure:"enabled"`
	// Transport 传输方式：sse / websocket。取值非法时回落 sse（对齐 Java
	// MessageTransportEnum.of 找不到即返回 SSE），故不在 validate 里报错。
	Transport string `mapstructure:"transport"`
	// Path 统一访问路径。
	Path string `mapstructure:"path"`
	// SSETimeoutSeconds SSE 连接超时(秒)，超时后服务端主动结束该流，由前端重连。
	// 对应 Java sse-timeout（那边单位是毫秒）。
	SSETimeoutSeconds int `mapstructure:"sseTimeoutSeconds"`
	// HeartbeatSeconds 心跳间隔(秒)，同时用于剔除失效连接，对应 Java heartbeat-interval。
	HeartbeatSeconds int `mapstructure:"heartbeatSeconds"`
	// SendBuffer 单条连接的待发送消息缓冲条数。
	//
	// 缓冲写满即断开该连接：推送是可丢的通知，不能让一个卡死的慢客户端
	// 把广播协程堵住（Java 侧 ConcurrentWebSocketSessionDecorator 的
	// bufferSizeLimit 同为此意，只是它按字节数算）。
	SendBuffer int `mapstructure:"sendBuffer"`
}

// DefaultPush 返回默认配置。
func DefaultPush() PushConfig {
	return PushConfig{
		Enabled:           true,
		Transport:         PushTransportSSE, // 对齐 Java MessageProperties 的默认值
		Path:              defaultPushPath,
		SSETimeoutSeconds: 86400,
		HeartbeatSeconds:  60,
		SendBuffer:        32,
	}
}

// IsWebSocket 是否走 WebSocket 传输。
// 非 websocket 一概视为 sse，对齐 Java MessageTransportEnum.of 的兜底。
func (c PushConfig) IsWebSocket() bool {
	return strings.EqualFold(c.Transport, PushTransportWebSocket)
}

// SSETimeout SSE 连接超时。
func (c PushConfig) SSETimeout() time.Duration {
	return time.Duration(c.SSETimeoutSeconds) * time.Second
}

// Heartbeat 心跳间隔。
func (c PushConfig) Heartbeat() time.Duration {
	return time.Duration(c.HeartbeatSeconds) * time.Second
}

// setDefaults 把默认值铺给 viper。
func (c PushConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("push.enabled", c.Enabled)
	v.SetDefault("push.transport", c.Transport)
	v.SetDefault("push.path", c.Path)
	v.SetDefault("push.sseTimeoutSeconds", c.SSETimeoutSeconds)
	v.SetDefault("push.heartbeatSeconds", c.HeartbeatSeconds)
	v.SetDefault("push.sendBuffer", c.SendBuffer)
}

// validate 校验推送配置。
func (c PushConfig) validate() error {
	// 关闭时其余项无需校验。
	if !c.Enabled {
		return nil
	}
	if c.Path == "" {
		return errMissing("push.path")
	}
	// 相对路径注册到 gin 会 panic，且前端拼的是绝对路径。
	if !strings.HasPrefix(c.Path, "/") {
		return errInvalid("push.path", "必须以 / 开头")
	}
	if c.SSETimeoutSeconds <= 0 {
		return errInvalid("push.sseTimeoutSeconds", "必须为正整数")
	}
	if c.HeartbeatSeconds <= 0 {
		return errInvalid("push.heartbeatSeconds", "必须为正整数")
	}
	if c.SendBuffer <= 0 {
		return errInvalid("push.sendBuffer", "必须为正整数")
	}
	return nil
}
