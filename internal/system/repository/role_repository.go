package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// RoleRepository sys_role 数据访问。
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 构造角色 repository。
func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// SelectRolesByUserId 按用户ID查其关联角色（对应 Java SysRoleMapper#selectRolesByUserId）。
// 经 sys_user_role 关联，sys_role 的逻辑删除由实体自动过滤；不按角色状态过滤（与 Java 一致）。
func (r *RoleRepository) SelectRolesByUserId(ctx context.Context, userID int64) ([]*model.SysRole, error) {
	if userID <= 0 {
		return nil, nil
	}

	var roles []*model.SysRole
	err := r.db.WithContext(ctx).
		Joins("JOIN sys_user_role sur ON sur.role_id = sys_role.role_id").
		Where("sur.user_id = ?", userID).
		Order("sys_role.role_sort").
		Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 角色失败: %w", userID, err)
	}
	return roles, nil
}
