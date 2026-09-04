package bo

// SysRoleQueryBo 角色列表查询条件（query 参数）。
//
// 与 SysRoleBo 分开而非复用：查询条件全部可选，而 SysRoleBo 的 binding:"required"
// 是新增场景的契约。Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则。
type SysRoleQueryBo struct {
	RoleID int64 `form:"roleId"`
	// RoleName 角色名称，LIKE 模糊匹配。
	RoleName string `form:"roleName"`
	// RoleKey 角色权限字符，LIKE 模糊匹配。
	RoleKey string `form:"roleKey"`
	// Status 角色状态，精确匹配。
	Status string `form:"status"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
