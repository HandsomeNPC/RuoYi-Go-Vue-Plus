package bo

// SysNoticeQueryBo 通知公告列表查询条件（query 参数）。
//
// 与 SysNoticeBo 分开而非复用：查询条件全部可选，而 SysNoticeBo 的
// binding:"required,max=50" 是新增/修改场景的契约。Go 的 binding tag 没有
// 校验分组概念，一个结构体只能有一套规则，故按用途拆型。
//
// 字段只取前端 querySchema 真正用到的三项（对齐 Java buildQueryWrapper），
// 不含创建时间区间——前端没有该筛选框，Java 侧也没有。
type SysNoticeQueryBo struct {
	NoticeTitle string `form:"noticeTitle"`
	// NoticeType 公告类型（1通知 2公告）。
	NoticeType string `form:"noticeType"`
	// CreateByName 创建人账号，按精确匹配。
	//
	// sys_notice 只存 create_by(用户ID)，故 service 层先把账号换成用户ID
	// 再交给 repository 过滤（对齐 Java 先查 userMapper 再 eq createBy）。
	CreateByName string `form:"createByName"`
}
