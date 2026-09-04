package model

// SysRoleDept 角色与部门关联表（sys_role_dept）。
// 复合主键（role_id, dept_id），无自增、无审计字段。
type SysRoleDept struct {
	RoleID int64 `gorm:"column:role_id;primaryKey" json:"roleId"`
	DeptID int64 `gorm:"column:dept_id;primaryKey" json:"deptId"`
}

// TableName 显式指定表名。
func (SysRoleDept) TableName() string {
	return "sys_role_dept"
}
