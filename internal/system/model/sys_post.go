package model

// SysPost 岗位表（sys_post），对应 Java org.dromara.system.domain.SysPost。
type SysPost struct {
	PostID       int64  `gorm:"column:post_id;primaryKey" json:"postId"`
	DeptID       int64  `gorm:"column:dept_id" json:"deptId"`
	PostCode     string `gorm:"column:post_code" json:"postCode"`
	PostName     string `gorm:"column:post_name" json:"postName"`
	PostCategory string `gorm:"column:post_category" json:"postCategory"`
	PostSort     int    `gorm:"column:post_sort" json:"postSort"`
	// Status 状态（0正常 1停用）。
	Status string `gorm:"column:status" json:"status"`
	// DelFlag 删除标志（0存在 1删除）。
	DelFlag string `gorm:"column:del_flag" json:"-"`
	Remark  string `gorm:"column:remark" json:"remark"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysPost) TableName() string {
	return "sys_post"
}
