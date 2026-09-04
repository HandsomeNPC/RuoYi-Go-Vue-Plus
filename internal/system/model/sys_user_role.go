package model

// SysUserRole 用户与角色关联表（sys_user_role）。
// 复合主键（user_id, role_id），无自增、无审计字段。
type SysUserRole struct {
	UserID int64 `gorm:"column:user_id;primaryKey" json:"userId"`
	RoleID int64 `gorm:"column:role_id;primaryKey" json:"roleId"`
}

// TableName 显式指定表名。
func (SysUserRole) TableName() string {
	return "sys_user_role"
}
