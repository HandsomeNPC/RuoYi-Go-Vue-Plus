package dto

import "time"

// 推送消息类型默认值，对应 Java PushTypeEnum / PushSourceEnum 的标识串。
// 此处仅枚举 PushPayloadDTO 构造时所需的默认值，完整枚举语义可后续迁至 pkg/enum。
const (
	// pushTypeMessage 通用消息。
	pushTypeMessage = "message"
	// pushSourceBackend 后端系统消息。
	pushSourceBackend = "backend"
)

// PushPayloadDTO 推送给前端的统一消息体，对应 Java
// org.dromara.system.api.domain.PushPayloadDTO。
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

// NewPushPayload 构建推送消息体，缺省消息类型与来源时使用系统默认值，
// 对应 Java PushPayloadDTO.of(String, String, String, Object)。
func NewPushPayload(typ, source, message string, data any) *PushPayloadDTO {
	if typ == "" {
		typ = pushTypeMessage
	}
	if source == "" {
		source = pushSourceBackend
	}
	return &PushPayloadDTO{
		Type:      typ,
		Source:    source,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewPushPayloadWithPath 构建带前端跳转路径的推送消息体，
// 对应 Java PushPayloadDTO.of(PushTypeEnum, PushSourceEnum, String, Object, String)。
// typ 与 source 传入空串即取系统默认值，等价于 Java 传 null。
func NewPushPayloadWithPath(typ, source, message string, data any, path string) *PushPayloadDTO {
	payload := NewPushPayload(typ, source, message, data)
	payload.Path = path
	return payload
}
