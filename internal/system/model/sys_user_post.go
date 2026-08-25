package model

// SysUserPost 用户与岗位关联表（sys_user_post），对应 Java org.dromara.system.domain.SysUserPost。
// 复合主键（user_id, post_id），无自增、无审计字段。
type SysUserPost struct {
	UserID int64 `gorm:"column:user_id;primaryKey" json:"userId"`
	PostID int64 `gorm:"column:post_id;primaryKey" json:"postId"`
}

// TableName 显式指定表名。
func (SysUserPost) TableName() string {
	return "sys_user_post"
}
