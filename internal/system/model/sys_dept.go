package model

// SysDept 部门表（sys_dept），对应 Java org.dromara.system.domain.SysDept。
type SysDept struct {
	DeptID       int64  `gorm:"column:dept_id;primaryKey" json:"deptId"`
	ParentID     int64  `gorm:"column:parent_id" json:"parentId"`
	DeptName     string `gorm:"column:dept_name" json:"deptName"`
	DeptCategory string `gorm:"column:dept_category" json:"deptCategory"`
	OrderNum     int    `gorm:"column:order_num" json:"orderNum"`
	Leader       int64  `gorm:"column:leader" json:"leader"`
	Phone        string `gorm:"column:phone" json:"phone"`
	Email        string `gorm:"column:email" json:"email"`
	// Status 部门状态（0正常 1停用）。
	Status string `gorm:"column:status" json:"status"`
	// DelFlag 删除标志（0存在 1删除）。
	DelFlag   string `gorm:"column:del_flag" json:"-"`
	Ancestors string `gorm:"column:ancestors" json:"ancestors"`

	// Children 子部门，非表字段，构建部门树时由 service 层填充。
	Children []*SysDept `gorm:"-" json:"children,omitempty"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysDept) TableName() string {
	return "sys_dept"
}
