package bo

// SysUserQueryBo 用户列表查询条件（query 参数）。
//
// 同时承载用户管理列表（list/export）与角色授权页（allocatedList/unallocatedList）两套过滤。
// 字段全部可选，与 SysUserBo 分开：后者 binding:"required" 是新增场景的契约，
// Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则。
type SysUserQueryBo struct {
	// UserName 用户名，LIKE 模糊匹配。
	UserName string `form:"userName"`
	// NickName 用户昵称，LIKE 模糊匹配。
	NickName string `form:"nickName"`
	// Status 账号状态，精确匹配。
	Status string `form:"status"`
	// PhoneNumber 手机号，LIKE 模糊匹配。
	PhoneNumber string `form:"phoneNumber"`
	// RoleID 角色 ID，allocatedList 上精确匹配 r.role_id；
	// unallocatedList 上用于先取已分配用户集合再 NOT IN 排除。
	RoleID int64 `form:"roleId"`
	// UserID 用户 ID，精确匹配（工作流按用户范围审批场景用）。
	UserID int64 `form:"userId"`
	// UserIDs 用户ID串，逗号分隔，命中即 IN。
	UserIDs string `form:"userIds"`
	// ExcludeUserIDs 排除用户ID串，逗号分隔，命中即 NOT IN。
	ExcludeUserIDs string `form:"excludeUserIds"`
	// DeptID 部门ID，service 解析成「自身+全部子部门」写回 DeptIDs 供 IN 过滤。
	DeptID int64 `form:"deptId"`
	// DeptIDs 解析后的部门ID集合，不接 query，由 service 按 DeptID 填充。
	DeptIDs []int64 `form:"-"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
