package dto

import (
	"time"

	"ruoyi-go-vue-plus/pkg/constant"
)

// PushPayloadDTO 推送给前端的统一消息体。
type PushPayloadDTO struct {
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

// NewPushPayload 构建推送消息体，缺省消息类型与来源时使用系统默认值。
func NewPushPayload(typ, source, message string, data any) *PushPayloadDTO {
	if typ == "" {
		typ = constant.PushTypeMessage
	}
	if source == "" {
		source = constant.PushSourceBackend
	}
	return &PushPayloadDTO{
		Type:      typ,
		Source:    source,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewPushPayloadWithPath 构建带前端跳转路径的推送消息体。
// typ 与 source 传入空串即取系统默认值。
func NewPushPayloadWithPath(typ, source, message string, data any, path string) *PushPayloadDTO {
	payload := NewPushPayload(typ, source, message, data)
	payload.Path = path
	return payload
}
