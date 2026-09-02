package bo

// SysUserQueryBo 用户列表查询条件（query 参数）。
//
// 目前只承载角色授权页 allocatedList/unallocatedList 用到的字段（对照 Java
// SysUserMapper#buildUserRoleJoinWrapper）。用户管理列表 CRUD 落地后可在此扩展，
// 届时把新增字段补进 applyUserAuthQuery 即可，不必另起一型。
type SysUserQueryBo struct {
	// UserName 用户名，LIKE 模糊匹配（对齐 Java likeIfText）。
	UserName string `form:"userName"`
	// Status 账号状态，精确匹配。
	Status string `form:"status"`
	// PhoneNumber 手机号，LIKE 模糊匹配。
	PhoneNumber string `form:"phoneNumber"`
	// RoleID 角色 ID，allocatedList 上精确匹配 r.role_id；
	// unallocatedList 上用于先取已分配用户集合再 NOT IN 排除。
	RoleID int64 `form:"roleId"`
}
