package model

import "ruoyi-go-vue-plus/pkg/repository"

// SysRole 角色表（sys_role）。
type SysRole struct {
	RoleID   int64  `gorm:"column:role_id;primaryKey" json:"roleId"`
	RoleName string `gorm:"column:role_name" json:"roleName"`
	RoleKey  string `gorm:"column:role_key" json:"roleKey"`
	RoleSort int    `gorm:"column:role_sort" json:"roleSort"`
	// DataScope 数据范围（1全部 2自定 3本部门 4本部门及以下 5仅本人 6部门及以下或本人）。
	DataScope string `gorm:"column:data_scope" json:"dataScope"`
	// MenuCheckStrictly 菜单树选择项是否关联显示（false不关联 true关联）。
	MenuCheckStrictly bool `gorm:"column:menu_check_strictly" json:"menuCheckStrictly"`
	// DeptCheckStrictly 部门树选择项是否关联显示（false不关联 true关联）。
	DeptCheckStrictly bool `gorm:"column:dept_check_strictly" json:"deptCheckStrictly"`
	// Status 角色状态（0正常 1停用）。
	Status string `gorm:"column:status" json:"status"`
	Remark string `gorm:"column:remark" json:"remark"`

	// LogicDelete 提供 del_flag 逻辑删除，查询/更新自动过滤已删除记录。
	repository.LogicDelete
	BaseEntity
}

// TableName 显式指定表名。
func (SysRole) TableName() string {
	return "sys_role"
}
