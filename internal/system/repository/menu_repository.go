package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/pkg/constant"
)

// MenuRepository sys_menu 数据访问。
type MenuRepository struct {
	db *gorm.DB
}

// NewMenuRepository 构造菜单 repository。
func NewMenuRepository(db *gorm.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

// RoleMenuPerm 角色-菜单权限投影行，对应 Java selectMenuPermsByRoleIds 的 (roleId, perms)。
// 非表实体，仅用于联表查询结果承载。
type RoleMenuPerm struct {
	RoleID int64  `gorm:"column:role_id"`
	Perms  string `gorm:"column:perms"`
}

// SelectMenuPermsByUserId 按用户ID查菜单权限标识（对应 Java SysMenuMapper#selectMenuPermsByUserId）。
// 路径：sys_menu m ← sys_role_menu srm ← sys_user_role sur（sur.role_id=srm.role_id）→ sys_role sr。
// 仅取正常角色（status=0、未删除）且 perms 非空的去重标识。
func (r *MenuRepository) SelectMenuPermsByUserId(ctx context.Context, userID int64) ([]string, error) {
	if userID <= 0 {
		return nil, nil
	}

	var rows []RoleMenuPerm
	err := r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Distinct("m.perms").
		Joins("JOIN sys_role_menu srm ON srm.menu_id = m.menu_id").
		Joins("JOIN sys_user_role sur ON sur.role_id = srm.role_id").
		Joins("JOIN sys_role sr ON sr.role_id = srm.role_id").
		Where("sur.user_id = ?", userID).
		Where("sr.status = ?", constant.StatusNormal).
		Where("sr.del_flag = ?", constant.StatusNormal). // del_flag '0' 未删除
		Where("m.perms IS NOT NULL AND m.perms <> ''").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 菜单权限失败: %w", userID, err)
	}

	out := make([]string, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].Perms)
	}
	return out, nil
}

// SelectMenuPermsByRoleIds 按角色ID集合批量查权限（对应 Java SysMenuMapper#selectMenuPermsByRoleIds）。
// 返回 (roleId, perms) 行序列，由 service 汇总成 map[roleId][]perms。
func (r *MenuRepository) SelectMenuPermsByRoleIds(ctx context.Context, roleIDs []int64) ([]RoleMenuPerm, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	var rows []RoleMenuPerm
	err := r.db.WithContext(ctx).
		Table("sys_role_menu AS srm").
		Select("srm.role_id, m.perms").
		Distinct().
		Joins("JOIN sys_menu m ON m.menu_id = srm.menu_id").
		Joins("JOIN sys_role sr ON sr.role_id = srm.role_id").
		Where("srm.role_id IN ?", roleIDs).
		Where("sr.status = ?", constant.StatusNormal).
		Where("sr.del_flag = ?", constant.StatusNormal).
		Where("m.perms IS NOT NULL AND m.perms <> ''").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 按角色 %v 查询菜单权限失败: %w", roleIDs, err)
	}
	return rows, nil
}
