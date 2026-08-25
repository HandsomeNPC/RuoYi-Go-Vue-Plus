package vo

// SysRoleMenuPermVo 角色菜单权限视图对象，对应 Java SysRoleMenuPermVo。
type SysRoleMenuPermVo struct {
	RoleID int64  `json:"roleId"`
	Perms  string `json:"perms"`
}
