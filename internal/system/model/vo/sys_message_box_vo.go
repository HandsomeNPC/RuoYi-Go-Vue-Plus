package vo

// SysMessageBoxVo 消息盒子视图对象，对应 Java SysMessageBoxVo。
type SysMessageBoxVo struct {
	SystemList   []*SysMessageVo `json:"systemList"`
	NoticeList   []*SysMessageVo `json:"noticeList"`
	WorkflowList []*SysMessageVo `json:"workflowList"`
}
