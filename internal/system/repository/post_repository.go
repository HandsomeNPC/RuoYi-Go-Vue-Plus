package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// PostRepository sys_post 数据访问。
type PostRepository struct {
	db *gorm.DB
}

// NewPostRepository 构造岗位 repository。
func NewPostRepository(db *gorm.DB) *PostRepository {
	return &PostRepository{db: db}
}

// CountByDeptID 统计部门下的岗位数（对应 Java countPostByDeptId）。
func (r *PostRepository) CountByDeptID(ctx context.Context, deptID int64) (int64, error) {
	if deptID <= 0 {
		return 0, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysPost{}).
		Where("dept_id = ?", deptID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("repository: 统计部门 %d 下的岗位失败: %w", deptID, err)
	}
	return count, nil
}

// SelectPostsByUserId 按用户ID查其关联岗位（对应 Java SysPostMapper#selectPostsByUserId）。
// 经 sys_user_post 关联，sys_post 的逻辑删除由实体自动过滤；不按岗位状态过滤（与 Java 一致）。
func (r *PostRepository) SelectPostsByUserId(ctx context.Context, userID int64) ([]*model.SysPost, error) {
	if userID <= 0 {
		return nil, nil
	}

	var posts []*model.SysPost
	err := r.db.WithContext(ctx).
		Joins("JOIN sys_user_post sup ON sup.post_id = sys_post.post_id").
		Where("sup.user_id = ?", userID).
		Order("sys_post.post_sort").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 岗位失败: %w", userID, err)
	}
	return posts, nil
}
