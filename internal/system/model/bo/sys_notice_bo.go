package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysNoticeBo 通知公告业务对象（入参），对应 Java SysNoticeBo。
type SysNoticeBo struct {
	NoticeID    int64  `json:"noticeId"`
	NoticeTitle string `json:"noticeTitle" binding:"required,max=50"`
	// NoticeType 公告类型（1通知 2公告）。
	NoticeType    string `json:"noticeType"`
	NoticeContent string `json:"noticeContent"`
	// Status 公告状态（0正常 1关闭）。
	Status string `json:"status"`
	Remark string `json:"remark"`
	// CreateByName 创建人名称，不落 sys_notice，由 service 按登录态回填。
	CreateByName string `json:"createByName"`
}

// ToSysNotice 把 BO 转成实体。
func (b *SysNoticeBo) ToSysNotice() *systemmodel.SysNotice {
	if b == nil {
		return nil
	}
	return &systemmodel.SysNotice{
		NoticeID:      b.NoticeID,
		NoticeTitle:   b.NoticeTitle,
		NoticeType:    b.NoticeType,
		NoticeContent: b.NoticeContent,
		Status:        b.Status,
		Remark:        b.Remark,
	}
}
