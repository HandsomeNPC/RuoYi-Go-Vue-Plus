// Package push 消息推送：SSE 长连接会话管理、跨进程分发与 gin 接入端点。
//
// 发布端(publish.go)无状态，只需 Redis，供 system 的消息/公告服务直接调用；
// 订阅端(push.go/conn.go/handler.go)持有本进程的连接，只在注册了推送路由的
// 进程里 Init。
package push

import (
	"context"
	"fmt"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
)

// PushDTO 一次推送的目标与载荷。
// 它是 Redis 主题里流转的消息格式，字段名不能改。
type PushDTO struct {
	// UserIDs 目标用户ID列表，为空表示广播给全部在线用户。
	UserIDs []int64 `json:"userIds"`
	// Payload 推送消息体。
	Payload *PayloadDTO `json:"payload"`
}

// Publish 发布推送消息，userIDs 为空即广播给全部在线用户。
//
// 必须走 Redis 而不能直接写本地会话：nginx 后可能有多个实例，
// 用户连在哪个实例上不确定，只发本地会话会让其余实例上的用户收不到。
// 也正因如此本函数无状态——发布方(system 进程)不必 Init 会话管理器。
func Publish(ctx context.Context, userIDs []int64,
	payload *PayloadDTO) error {

	if payload == nil {
		return nil
	}

	body, err := jsonx.Marshal(PushDTO{UserIDs: userIDs, Payload: payload})
	if err != nil {
		return fmt.Errorf("push: 序列化推送消息失败: %w", err)
	}
	if err := redis.Client().Publish(ctx, MessageTopic, body).Err(); err != nil {
		return fmt.Errorf("push: 发布推送消息失败: %w", err)
	}
	return nil
}
