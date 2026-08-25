package model

// SysNotice 通知公告表（sys_notice），对应 Java org.dromara.system.domain.SysNotice。
type SysNotice struct {
	NoticeID    int64  `gorm:"column:notice_id;primaryKey" json:"noticeId"`
	NoticeTitle string `gorm:"column:notice_title" json:"noticeTitle"`
	// NoticeType 公告类型（1通知 2公告）。
	NoticeType    string `gorm:"column:notice_type" json:"noticeType"`
	NoticeContent string `gorm:"column:notice_content" json:"noticeContent"`
	// Status 公告状态（0正常 1关闭）。
	Status string `gorm:"column:status" json:"status"`
	Remark string `gorm:"column:remark" json:"remark"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysNotice) TableName() string {
	return "sys_notice"
}
