package vo

// SysUserInfoVo 用户信息视图对象，对应 Java SysUserInfoVo。
type SysUserInfoVo struct {
	User SysUserVo `json:"user"`
	// RoleIDs 角色ID列表，由 service 回填。
	RoleIDs []int64     `json:"roleIds"`
	Roles   []SysRoleVo `json:"roles"`
	// PostIDs 岗位ID列表，由 service 回填。
	PostIDs []int64     `json:"postIds"`
	Posts   []SysPostVo `json:"posts"`
}
