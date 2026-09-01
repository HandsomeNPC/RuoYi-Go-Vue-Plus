package bo

// SysMenuQueryBo 菜单列表查询条件（query 参数）。
//
// 与 SysMenuBo 分开而非复用：查询条件全部可选，而 SysMenuBo 的 binding:"required"
// （menuName/orderNum/menuType）是新增场景的契约。Go 的 binding tag 没有校验分组概念，
// 一个结构体只能有一套规则。
//
// 无创建时间区间：Java buildQueryWrapper 未提供，前端筛选表单也只有三项，
// 不构造无效的查询能力。
type SysMenuQueryBo struct {
	MenuName string `form:"menuName"`
	// Visible 显示状态（0显示 1隐藏）。
	Visible string `form:"visible"`
	// Status 菜单状态（0正常 1停用）。
	Status string `form:"status"`
	// MenuType 菜单类型（M目录 C菜单 F按钮）。
	MenuType string `form:"menuType"`
	// ParentID 取 0 视为不筛。Java 用 Long 的 null 区分"未传"与"显式传 0"，
	// Go 的 int64 两者都塌成 0；顶级菜单的 parent_id 恰是 0，故"只看顶级菜单"
	// 这一筛法在 Go 侧不可表达——前端未提供该入口，取不到可观测差异。
	ParentID int64 `form:"parentId"`
}
