package service

import (
	"context"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/pkg/constant"
)

// PermissionService 用户权限处理（对应 Java SysPermissionServiceImpl）。
type PermissionService struct{}

// PermissionSvcApp 包级实例。
var PermissionSvcApp = new(PermissionService)

// GetRolePermission 获取角色权限标识（对应 Java SysPermissionServiceImpl#getRolePermission）。
// 超级管理员拥有固定角色键，其余用户取其关联角色的 roleKey。
func (s *PermissionService) GetRolePermission(ctx context.Context, userID int64) ([]string, error) {
	if userID == constant.SuperAdminUserID {
		return []string{constant.SuperAdminRoleKey}, nil
	}
	return RoleSvcApp.SelectRolePermissionByUserId(ctx, userID)
}

// GetMenuPermission 获取菜单权限标识（对应 Java SysPermissionServiceImpl#getMenuPermission）。
// 超级管理员拥有全部权限（*:*:*），其余用户取其角色关联的菜单 perms。
func (s *PermissionService) GetMenuPermission(ctx context.Context, userID int64) ([]string, error) {
	if userID == constant.SuperAdminUserID {
		return []string{"*:*:*"}, nil
	}
	return MenuSvcApp.SelectMenuPermsByUserId(ctx, userID)
}

// GetDataScopeRoleMap 按权限标识汇总具备数据权限的角色集合
// （对应 Java SysPermissionServiceImpl#getDataScopeRoleMap(List<RoleDTO>)）。
// key 为权限标识，value 为拥有该权限的角色ID列表。
func (s *PermissionService) GetDataScopeRoleMap(ctx context.Context,
	roles []*systemdto.RoleDTO) (map[string][]int64, error) {
	if len(roles) == 0 {
		return map[string][]int64{}, nil
	}

	roleIDs := make([]int64, 0, len(roles))
	for i := range roles {
		roleIDs = append(roleIDs, roles[i].RoleID)
	}

	permsByRole, err := MenuSvcApp.SelectMenuPermsByRoleIds(ctx, roleIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]int64)
	for roleID, perms := range permsByRole {
		for _, perm := range perms {
			out[perm] = append(out[perm], roleID)
		}
	}
	return out, nil
}
