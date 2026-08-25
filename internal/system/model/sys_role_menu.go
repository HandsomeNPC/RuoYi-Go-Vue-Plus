package model

// SysRoleMenu 角色与菜单关联表（sys_role_menu），对应 Java org.dromara.system.domain.SysRoleMenu。
// 复合主键（role_id, menu_id），无自增、无审计字段。
type SysRoleMenu struct {
	RoleID int64 `gorm:"column:role_id;primaryKey" json:"roleId"`
	MenuID int64 `gorm:"column:menu_id;primaryKey" json:"menuId"`
}

// TableName 显式指定表名。
func (SysRoleMenu) TableName() string {
	return "sys_role_menu"
}
