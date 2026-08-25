package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysNoticeVo 通知公告视图对象，对应 Java SysNoticeVo。
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

// FromSysNotice 把实体转成 VO。
func FromSysNotice(n *systemmodel.SysNotice) *SysNoticeVo {
	if n == nil {
		return nil
	}
	return &SysNoticeVo{
		NoticeID:      n.NoticeID,
		NoticeTitle:   n.NoticeTitle,
		NoticeType:    n.NoticeType,
		NoticeContent: n.NoticeContent,
		Status:        n.Status,
		Remark:        n.Remark,
		CreateBy:      n.CreateBy,
		CreateTime:    n.CreateTime,
	}
}
