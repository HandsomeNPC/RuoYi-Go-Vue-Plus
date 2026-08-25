package bo

import systemmodel "ruoyi-go-vue-plus/internal/system/model"

// SysRoleBo 角色业务对象（入参），对应 Java SysRoleBo。
type SysRoleBo struct {
	RoleID   int64  `json:"roleId"`
	RoleName string `json:"roleName" binding:"required,max=30"`
	RoleKey  string `json:"roleKey" binding:"required,max=100"`
	RoleSort int    `json:"roleSort" binding:"required"`
	// DataScope 数据范围（1全部 2自定 3本部门 4本部门及以下 5仅本人 6部门及以下或本人）。
	DataScope         string `json:"dataScope"`
	MenuCheckStrictly bool   `json:"menuCheckStrictly"`
	DeptCheckStrictly bool   `json:"deptCheckStrictly"`
	// Status 角色状态（0正常 1停用）。
	Status string `json:"status"`
	Remark string `json:"remark"`
	// MenuIDs 菜单组，不落 sys_role，由 RoleService 写 sys_role_menu。
	MenuIDs []int64 `json:"menuIds"`
	// DeptIDs 部门组，不落 sys_role，由 RoleService 写 sys_role_dept。
	DeptIDs []int64 `json:"deptIds"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}

// ToSysRole 把 BO 转成实体。
func (b *SysRoleBo) ToSysRole() *systemmodel.SysRole {
	if b == nil {
		return nil
	}
	return &systemmodel.SysRole{
		RoleID:            b.RoleID,
		RoleName:          b.RoleName,
		RoleKey:           b.RoleKey,
		RoleSort:          b.RoleSort,
		DataScope:         b.DataScope,
		MenuCheckStrictly: b.MenuCheckStrictly,
		DeptCheckStrictly: b.DeptCheckStrictly,
		Status:            b.Status,
		Remark:            b.Remark,
	}
}
