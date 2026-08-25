package model

// SysMessage 消息记录表（sys_message），对应 Java org.dromara.system.domain.SysMessage。
type SysMessage struct {
	MessageID int64  `gorm:"column:message_id;primaryKey" json:"messageId"`
	Category  string `gorm:"column:category" json:"category"`
	Type      string `gorm:"column:type" json:"type"`
	Source    string `gorm:"column:source" json:"source"`
	Title     string `gorm:"column:title" json:"title"`
	Message   string `gorm:"column:message" json:"message"`
	Content   string `gorm:"column:content" json:"content"`
	DataJSON  string `gorm:"column:data_json" json:"dataJson"`
	Path      string `gorm:"column:path" json:"path"`
	// SendUserIDs 目标用户ID串，0 表示全局。
	SendUserIDs string `gorm:"column:send_user_ids" json:"sendUserIds"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysMessage) TableName() string {
	return "sys_message"
}
