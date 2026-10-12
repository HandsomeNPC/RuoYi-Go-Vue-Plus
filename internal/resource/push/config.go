package push

import (
	"fmt"
	"strings"
	"time"
)

// 默认值与校验在 Init 兜底，不挂在结构体方法上。
const (
	// defaultPath 推送端点路径的相对部分，注册时由调用方拼上 prefix。
	//
	// 微服务版 nacos 把 message.path 覆盖成 /message，网关对 /resource/**
	// 做 StripPrefix=1 剥前缀，故后端只注册裸 /message。单体/直连场景由 prefix
	// （standalone 传 /resource）拼成完整 /resource/message。
	defaultPath = "/message"
	// defaultSSETimeoutSeconds 对齐原项目的 86400000 毫秒。
	defaultSSETimeoutSeconds = 86400
	defaultHeartbeatSeconds  = 60
	defaultSendBuffer        = 32
)

// PushConfig 消息推送配置，对应 yaml 的 push 段。
//
// 未暴露 allowedOrigins：跨域已由 CORS 中间件统一管，握手期再加一套白名单只会两处打架。
type PushConfig struct {
	// Enabled 是否启用消息推送。关闭时不注册推送路由。
	//
	// 缺省为 false：bool 零值无法区分「未配置」与「显式 false」，
	// 而「配置缺失 → 功能关闭」是更安全的失败方向。
	Enabled bool `mapstructure:"enabled"`
	// Path 统一访问路径。
	Path string `mapstructure:"path"`
	// SSETimeoutSeconds SSE 连接超时(秒)，超时后服务端主动结束该流，由前端重连。
	// 注意原项目单位是毫秒。
	SSETimeoutSeconds int `mapstructure:"sseTimeoutSeconds"`
	// HeartbeatSeconds 心跳间隔(秒)，同时用于剔除失效连接。
	HeartbeatSeconds int `mapstructure:"heartbeatSeconds"`
	// SendBuffer 单条连接的待发送消息缓冲条数。
	//
	// 缓冲写满即断开该连接：推送是可丢的通知，不能让一个卡死的慢客户端
	// 把广播协程堵住。
	SendBuffer int `mapstructure:"sendBuffer"`
}

// applyDefaults 就地铺兜底默认值。负数留给 validate 拦。
func (c *PushConfig) applyDefaults() {
	if c.Path == "" {
		c.Path = defaultPath
	}
	if c.SSETimeoutSeconds == 0 {
		c.SSETimeoutSeconds = defaultSSETimeoutSeconds
	}
	if c.HeartbeatSeconds == 0 {
		c.HeartbeatSeconds = defaultHeartbeatSeconds
	}
	if c.SendBuffer == 0 {
		c.SendBuffer = defaultSendBuffer
	}
}

// validate 校验推送配置。
//
// 关闭时直接放行：线上可能存在 enabled=false 且其余项无效的配置，
// 一并校验会让这些配置启动失败。
func (c PushConfig) validate() error {
	if !c.Enabled {
		return nil
	}
	// 相对路径注册到 gin 会 panic，且前端拼的是绝对路径。
	if !strings.HasPrefix(c.Path, "/") {
		return fmt.Errorf("push.path 必须以 / 开头")
	}
	if c.SSETimeoutSeconds <= 0 {
		return fmt.Errorf("push.sseTimeoutSeconds 必须为正整数")
	}
	if c.HeartbeatSeconds <= 0 {
		return fmt.Errorf("push.heartbeatSeconds 必须为正整数")
	}
	if c.SendBuffer <= 0 {
		return fmt.Errorf("push.sendBuffer 必须为正整数")
	}
	return nil
}

// sseTimeout SSE 连接超时。
func (c PushConfig) sseTimeout() time.Duration {
	return time.Duration(c.SSETimeoutSeconds) * time.Second
}

// heartbeat 心跳间隔。
func (c PushConfig) heartbeat() time.Duration {
	return time.Duration(c.HeartbeatSeconds) * time.Second
}
