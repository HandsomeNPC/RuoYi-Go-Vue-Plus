// Package push WebSocket 消息推送：会话管理、跨进程分发与 gin 接入端点。
//
// 对照 Java ruoyi-common-push 模块，只落 websocket 分支（未移植 SSE）。
// 三个文件各管一层：session.go 管本进程的连接，hub.go 经 Redis 主题把消息
// 分发到所有实例，handler.go 是 gin 侧的握手入口。
package push

import (
	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
)

// PushDTO 一次推送的目标与载荷，对应 Java org.dromara.common.push.dto.PushDTO。
// 它是 Redis 主题里流转的消息格式，字段名须与 Java 侧保持一致。
type PushDTO struct {
	// UserIDs 目标用户ID列表，为空表示广播给全部在线用户
	// （对齐 Java PushDTO.broadcast 传 null 的语义）。
	UserIDs []int64 `json:"userIds"`
	// Payload 推送消息体。
	Payload *systemdto.PushPayloadDTO `json:"payload"`
}

// ToUsers 构建指定用户的推送（对应 Java PushDTO.of）。
func ToUsers(userIDs []int64, payload *systemdto.PushPayloadDTO) PushDTO {
	return PushDTO{UserIDs: userIDs, Payload: payload}
}

// Broadcast 构建广播推送（对应 Java PushDTO.broadcast）。
func Broadcast(payload *systemdto.PushPayloadDTO) PushDTO {
	return PushDTO{Payload: payload}
}
