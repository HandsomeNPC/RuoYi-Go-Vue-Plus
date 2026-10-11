package permission

import (
	"context"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/constant"
	domain "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/domain"
	menuservice "ruoyi-go-vue-plus/internal/system/service/menu"
	roleservice "ruoyi-go-vue-plus/internal/system/service/role"
)

// GetRolePermission 获取角色权限标识。
// 超级管理员拥有固定角色键，其余用户取其关联角色的 roleKey。
func GetRolePermission(ctx context.Context, userID int64) ([]string, error) {
	if userID == constant.SuperAdminUserID {
		return []string{constant.SuperAdminRoleKey}, nil
	}
	return roleservice.SelectRolePermissionByUserId(ctx, userID)
}

// GetMenuPermission 获取菜单权限标识。
// 超级管理员拥有全部权限（*:*:*），其余用户取其角色关联的菜单 perms。
func GetMenuPermission(ctx context.Context, userID int64) ([]string, error) {
	if userID == constant.SuperAdminUserID {
		return []string{"*:*:*"}, nil
	}
	return menuservice.SelectMenuPermsByUserId(ctx, userID)
}

// GetDataScopeRoleMap 按权限标识汇总具备数据权限的角色集合。
// key 为权限标识，value 为拥有该权限的角色ID列表。
func GetDataScopeRoleMap(ctx context.Context,
	roles []*domain.RoleDTO) (map[string][]int64, error) {
	if len(roles) == 0 {
		return map[string][]int64{}, nil
	}

	// 按 roles 顺序去重收集角色ID。
	roleIDs := make([]int64, 0, len(roles))
	seenRole := make(map[int64]struct{}, len(roles))
	for i := range roles {
		if roles[i] == nil {
			continue
		}
		roleID := roles[i].RoleID
		if _, dup := seenRole[roleID]; dup {
			continue
		}
		seenRole[roleID] = struct{}{}
		roleIDs = append(roleIDs, roleID)
	}

	permsByRole, err := menuservice.SelectMenuPermsByRoleIds(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	return invertPermsByRole(roleIDs, permsByRole), nil
}

// invertPermsByRole 把 map[roleID]perms 翻转成 map[perm]roleIDs。
// 必须按 roleIDs 顺序遍历而非 range permsByRole：Go map 迭代顺序随机，
// 直接 range 会让同一用户每次登录得到的角色ID列表顺序漂移，故需稳定插入序。
func invertPermsByRole(roleIDs []int64, permsByRole map[int64][]string) map[string][]int64 {
	out := make(map[string][]int64)
	for _, roleID := range roleIDs {
		for _, perm := range permsByRole[roleID] {
			out[perm] = append(out[perm], roleID)
		}
	}
	return out
}
