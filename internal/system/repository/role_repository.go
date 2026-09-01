package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// ErrRoleNotFound 角色不存在。
var ErrRoleNotFound = errors.New("repository: 角色不存在")

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

// SelectByID 按主键查角色，不存在时返回 ErrRoleNotFound。
func (r *RoleRepository) SelectByID(ctx context.Context, roleID int64) (*model.SysRole, error) {
	if roleID <= 0 {
		return nil, ErrRoleNotFound
	}

	var role model.SysRole
	err := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("repository: 查询角色 id=%d 失败: %w", roleID, err)
	}
	return &role, nil
}

// ExistsRoleMenuByMenuID 判断菜单是否已分配给任何角色（对应 Java checkMenuExistRole）。
func (r *RoleRepository) ExistsRoleMenuByMenuID(ctx context.Context, menuID int64) (bool, error) {
	if menuID <= 0 {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysRoleMenu{}).
		Where("menu_id = ?", menuID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询菜单 %d 的角色分配失败: %w", menuID, err)
	}
	return count > 0, nil
}

// DeleteRoleMenuByMenuIDs 按菜单主键批量清理角色-菜单关联（对应 Java SysRoleMenuMapper#deleteByMenuIds）。
// sys_role_menu 无 del_flag，这是物理删除。
func (r *RoleRepository) DeleteRoleMenuByMenuIDs(ctx context.Context, menuIDs []int64) (int64, error) {
	if len(menuIDs) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Where("menu_id IN ?", menuIDs).
		Delete(&model.SysRoleMenu{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理菜单 %v 的角色关联失败: %w", menuIDs, res.Error)
	}
	return res.RowsAffected, nil
}
