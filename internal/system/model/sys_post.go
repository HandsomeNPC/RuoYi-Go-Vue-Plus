package model

import "ruoyi-go-vue-plus/pkg/repository"

// SysPost 岗位表（sys_post）。
type SysPost struct {
	PostID       int64  `gorm:"column:post_id;primaryKey" json:"postId"`
	DeptID       int64  `gorm:"column:dept_id" json:"deptId"`
	PostCode     string `gorm:"column:post_code" json:"postCode"`
	PostName     string `gorm:"column:post_name" json:"postName"`
	PostCategory string `gorm:"column:post_category" json:"postCategory"`
	PostSort     int    `gorm:"column:post_sort" json:"postSort"`
	// Status 状态（0正常 1停用）。
	Status string `gorm:"column:status" json:"status"`
	Remark string `gorm:"column:remark" json:"remark"`

	// LogicDelete 提供 del_flag 逻辑删除，查询/更新自动过滤已删除记录。
	repository.LogicDelete
	BaseEntity
}

// TableName 显式指定表名。
func (SysPost) TableName() string {
	return "sys_post"
}
