package vo

import (
	"time"
)

// SysNoticeVo 通知公告视图对象。
type SysNoticeVo struct {
	NoticeID    int64  `json:"noticeId"`
	NoticeTitle string `json:"noticeTitle"`
	// NoticeType 公告类型（1通知 2公告）。
	NoticeType    string `json:"noticeType"`
	NoticeContent string `json:"noticeContent"`
	// Status 公告状态（0正常 1关闭）。
	Status   string `json:"status"`
	Remark   string `json:"remark"`
	CreateBy int64  `json:"createBy"`
	// CreateByName 创建人名称，由翻译层按 USER_ID_TO_NAME 从 CreateBy 回填。
	CreateByName string     `json:"createByName"`
	CreateTime   *time.Time `json:"createTime"`
}
