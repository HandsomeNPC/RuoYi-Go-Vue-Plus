// Package push 消息推送：会话管理、跨进程分发与 gin 接入端点。
//
// websocket 与 SSE 两种长连接共用本包的连接管理：session.go 管本进程的
// 连接，hub.go 经 Redis 主题把消息分发到所有实例，handler.go 是 gin 侧
// 的握手入口，ws_conn.go / sse_conn.go 分别实现两种连接的帧协议。
package push

import (
	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
)

// PushDTO 一次推送的目标与载荷。
// 它是 Redis 主题里流转的消息格式，字段名不能改。
type PushDTO struct {
	// UserIDs 目标用户ID列表，为空表示广播给全部在线用户
	// （nil 表示广播）。
	UserIDs []int64 `json:"userIds"`
	// Payload 推送消息体。
	Payload *systemdto.PushPayloadDTO `json:"payload"`
}

// ToUsers 构建指定用户的推送。
func ToUsers(userIDs []int64, payload *systemdto.PushPayloadDTO) PushDTO {
	return PushDTO{UserIDs: userIDs, Payload: payload}
}

// Broadcast 构建广播推送。
func Broadcast(payload *systemdto.PushPayloadDTO) PushDTO {
	return PushDTO{Payload: payload}
}
