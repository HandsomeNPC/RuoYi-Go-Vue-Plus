package push

import (
	"time"
)

// MessageTopic 跨进程推送的 Redis 订阅主题。
//
// 主题名不能改：nginx 后可能挂多个 system 实例，且需与原实现混部时互通。
const MessageTopic = "global:message"

// 推送消息类型。
//
// 只有一个标识串、无附加信息，故放常量而非做成枚举类型。
const (
	// TypeMessage 通用消息。
	TypeMessage = "message"
	// TypeNotice 通知公告。
	TypeNotice = "notice"
	// TypeLLM 大模型消息，不入消息盒子。
	TypeLLM = "llm"
	// TypeCustom 客户端自定义消息。
	TypeCustom = "custom"
)

// 推送消息来源。
const (
	// SourceBackend 后端系统消息。
	SourceBackend = "backend"
	// SourceNotice 通知公告。
	SourceNotice = "notice"
	// SourceWorkflow 工作流。
	SourceWorkflow = "workflow"
	// SourceLLM 大模型。
	SourceLLM = "llm"
	// SourceClient 客户端消息。
	SourceClient = "client"
)

// PayloadDTO 推送给前端的统一消息体。
//
// 它是 MessageTopic 这份 Redis 消息的 JSON 契约：system 的消息/公告服务发布、
// 订阅端分发给 SSE 连接，故字段名不能改。
type PayloadDTO struct {
	// MessageID 消息记录ID。
	MessageID int64 `json:"messageId"`
	// Type 消息类型。
	Type string `json:"type"`
	// Source 消息来源。
	Source string `json:"source"`
	// Message 文本消息。
	Message string `json:"message"`
	// Data 扩展数据。
	Data any `json:"data"`
	// Path 前端跳转路径。
	Path string `json:"path"`
	// Timestamp 时间戳。
	Timestamp int64 `json:"timestamp"`
}

// newPayload 构建推送消息体，缺省消息类型与来源时使用系统默认值。
func newPayload(typ, source, message string, data any) *PayloadDTO {
	if typ == "" {
		typ = TypeMessage
	}
	if source == "" {
		source = SourceBackend
	}
	return &PayloadDTO{
		Type:      typ,
		Source:    source,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewPayloadWithPath 构建带前端跳转路径的推送消息体。
// typ 与 source 传入空串即取系统默认值。
func NewPayloadWithPath(typ, source, message string, data any, path string) *PayloadDTO {
	payload := newPayload(typ, source, message, data)
	payload.Path = path
	return payload
}
