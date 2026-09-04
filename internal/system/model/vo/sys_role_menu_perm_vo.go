package vo

// SysRoleMenuPermVo 角色菜单权限视图对象。
type SysRoleMenuPermVo struct {
	RoleID int64  `json:"roleId"`
	Perms  string `json:"perms"`
}
