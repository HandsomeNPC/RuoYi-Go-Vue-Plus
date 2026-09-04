package service

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/push"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// 消息盒子的展示口径：每类最多 boxLimit 条、最近 boxDays 天。
const (
	// boxLimit 每个分类最多展示的条数。
	boxLimit = 100
	// boxDays 只展示最近多少天内的消息。
	boxDays = 30
)

// 消息盒子的标题文案。
const (
	titleSystem   = "系统消息"
	titleNotice   = "通知公告消息"
	titleWorkflow = "工作流消息"
)

// MessageService 消息记录业务逻辑。
type MessageService struct{}

var MessageSvcApp = new(MessageService)

// QueryMessageBox 查当前用户的消息盒子。
// 按系统消息 / 通知公告 / 工作流三类分别返回。
func (s *MessageService) QueryMessageBox(ctx context.Context,
	userID int64) (*vo.SysMessageBoxVo, error) {

	box := new(vo.SysMessageBoxVo)
	// 三个分类分别查：每类各自限流 100 条，单查才能让 LIMIT 落到 SQL 上。
	for _, item := range []struct {
		category string
		dest     *[]*vo.SysMessageVo
	}{
		{constant.MessageCategorySystem, &box.SystemList},
		{constant.MessageCategoryNotice, &box.NoticeList},
		{constant.MessageCategoryWorkflow, &box.WorkflowList},
	} {
		rows, err := s.selectBoxList(ctx, item.category, userID)
		if err != nil {
			return nil, err
		}
		*item.dest = rows
	}
	return box, nil
}

// selectBoxList 查单个分类的消息并转 VO。
func (s *MessageService) selectBoxList(ctx context.Context, category string,
	userID int64) ([]*vo.SysMessageVo, error) {

	since := time.Now().AddDate(0, 0, -boxDays)
	rows, err := repository.NewMessageRepository(database.DB()).
		SelectBoxList(ctx, category, userID, since, boxLimit)
	if err != nil {
		return nil, err
	}

	out := vo.Conv.ConvertToSysMessageVoList(rows)
	// Data 是 data_json 反序列化后的对象，goverter 映射不了（类型不同），单独回填。
	for i, row := range rows {
		out[i].Data = parseMessageData(row.DataJSON)
	}
	// 空切片而非 nil：让 JSON 出参是 [] 而不是 null，前端可直接 .length。
	if out == nil {
		out = []*vo.SysMessageVo{}
	}
	return out, nil
}

// parseMessageData 反序列化扩展数据。
// 解析失败按空处理：data_json 是展示用的附加信息，坏了一条不该让整个消息盒子失败。
func parseMessageData(dataJSON string) any {
	if strings.TrimSpace(dataJSON) == "" {
		return nil
	}
	var data any
	if err := jsonx.Unmarshal([]byte(dataJSON), &data); err != nil {
		log.Printf("[message] 解析扩展数据失败: %v", err)
		return nil
	}
	return data
}

// PublishAll 广播消息给全部用户：先落消息盒子再推送。
func (s *MessageService) PublishAll(ctx context.Context, payload *dto.PushPayloadDTO) error {
	return s.publish(ctx, nil, payload)
}

// PublishUsers 发布消息给指定用户。
func (s *MessageService) PublishUsers(ctx context.Context, userIDs []int64,
	payload *dto.PushPayloadDTO) error {

	return s.publish(ctx, userIDs, payload)
}

// publish 统一的「存储 + 推送」流程。
//
// 落库失败要上抛：消息盒子查不到就等于消息丢了。推送失败只记日志——
// 消息已经进盒子，用户下次打开就能看到，不值得让调用方（如公告新增）整体失败。
func (s *MessageService) publish(ctx context.Context, userIDs []int64,
	payload *dto.PushPayloadDTO) error {

	if payload == nil {
		return nil
	}
	if err := s.storeMessage(ctx, userIDs, payload); err != nil {
		return err
	}
	if err := push.Publish(ctx, push.ToUsers(userIDs, payload)); err != nil {
		log.Printf("[message] 推送失败(消息已入库，不影响业务): %v", err)
	}
	return nil
}

// storeMessage 把消息写进消息盒子，并回填 payload.MessageID。
// 不入盒子的类型（如 LLM 流式消息）直接跳过。
func (s *MessageService) storeMessage(ctx context.Context, userIDs []int64,
	payload *dto.PushPayloadDTO) error {

	if !supportsMessageBox(payload) {
		return nil
	}

	msg := buildMessage(userIDs, payload)
	if err := repository.NewMessageRepository(database.DB()).Insert(ctx, msg); err != nil {
		return err
	}
	// 回填给调用方：前端拿 messageId 标记已读、跳详情。
	payload.MessageID = msg.MessageID
	return nil
}

// buildMessage 组装消息实体。
func buildMessage(userIDs []int64, payload *dto.PushPayloadDTO) *model.SysMessage {
	messageID := payload.MessageID
	// 调用方已指定则沿用，否则发号。
	if messageID == 0 {
		messageID = snowflake.Next() // message_id 无 auto_increment
	}

	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。
	return &model.SysMessage{
		MessageID:   messageID,
		Category:    resolveCategory(payload),
		Type:        payload.Type,
		Source:      payload.Source,
		Title:       resolveTitle(payload),
		Message:     payload.Message,
		Content:     resolveContent(payload),
		DataJSON:    marshalMessageData(payload.Data),
		Path:        payload.Path,
		SendUserIDs: joinSendUserIDs(userIDs),
	}
}

// marshalMessageData 序列化扩展数据，失败时存空串。
// 与 parseMessageData 对称：扩展数据是附加信息，序列化不了不该阻断消息入库。
func marshalMessageData(data any) string {
	if data == nil {
		return ""
	}
	b, err := jsonx.Marshal(data)
	if err != nil {
		log.Printf("[message] 序列化扩展数据失败: %v", err)
		return ""
	}
	return string(b)
}

// joinSendUserIDs 拼接接收人串，空列表表示全局广播。
func joinSendUserIDs(userIDs []int64) string {
	if len(userIDs) == 0 {
		return constant.MessageGlobalUserIDs
	}
	parts := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// supportsMessageBox 判断该消息是否需要入消息盒子。
// 只有通用消息与通知公告入盒子，且排除大模型消息——LLM 是逐字流式下发的，
// 每个片段都存一条会瞬间把表写满。
func supportsMessageBox(payload *dto.PushPayloadDTO) bool {
	if payload == nil {
		return false
	}
	if payload.Type != constant.PushTypeMessage && payload.Type != constant.PushTypeNotice {
		return false
	}
	return payload.Type != constant.PushTypeLLM && payload.Source != constant.PushSourceLLM
}

// resolveCategory 按类型/来源推断消息分类。
func resolveCategory(payload *dto.PushPayloadDTO) string {
	switch {
	case payload.Type == constant.PushTypeNotice || payload.Source == constant.PushSourceNotice:
		return constant.MessageCategoryNotice
	case payload.Source == constant.PushSourceWorkflow:
		return constant.MessageCategoryWorkflow
	default:
		return constant.MessageCategorySystem
	}
}

// resolveTitle 按分类生成标题。
func resolveTitle(payload *dto.PushPayloadDTO) string {
	switch resolveCategory(payload) {
	case constant.MessageCategoryNotice:
		return titleNotice
	case constant.MessageCategoryWorkflow:
		return titleWorkflow
	default:
		return titleSystem
	}
}

// resolveContent 从扩展数据里取详细内容。
// 只认 map 形态的 data 里的 noticeContent 键，其余情况留空。
func resolveContent(payload *dto.PushPayloadDTO) string {
	data, ok := payload.Data.(map[string]any)
	if !ok {
		return ""
	}
	content, ok := data["noticeContent"].(string)
	if !ok {
		return ""
	}
	return content
}
