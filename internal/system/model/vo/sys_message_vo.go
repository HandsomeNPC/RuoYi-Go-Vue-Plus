package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysMessageVo 消息记录视图对象，对应 Java SysMessageVo。
type SysMessageVo struct {
	MessageID int64  `json:"messageId"`
	Category  string `json:"category"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Content   string `json:"content"`
	// Data 扩展数据，不落 sys_message，由 service 从 DataJSON 反序列化回填。
	Data       any        `json:"data"`
	Path       string     `json:"path"`
	CreateTime *time.Time `json:"createTime"`
}

// FromSysMessage 把实体转成 VO。
func FromSysMessage(m *systemmodel.SysMessage) *SysMessageVo {
	if m == nil {
		return nil
	}
	return &SysMessageVo{
		MessageID:  m.MessageID,
		Category:   m.Category,
		Type:       m.Type,
		Source:     m.Source,
		Title:      m.Title,
		Message:    m.Message,
		Content:    m.Content,
		Path:       m.Path,
		CreateTime: m.CreateTime,
	}
}
