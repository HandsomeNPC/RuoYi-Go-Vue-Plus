package service

import (
	"context"
	"errors"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// ErrRoleNotFound 角色不存在。
var ErrRoleNotFound = errors.New("service: 角色不存在")

// RoleService 角色业务逻辑。
type RoleService struct{}

// RoleSvcApp 包级实例。
var RoleSvcApp = new(RoleService)

// SelectRolesByUserId 按用户ID查角色列表（对应 Java SysRoleServiceImpl#selectRolesByUserId）。
func (s *RoleService) SelectRolesByUserId(ctx context.Context, userID int64) ([]*vo.SysRoleVo, error) {
	roles, err := repository.NewRoleRepository(database.DB()).SelectRolesByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysRoleVoList(roles), nil
}

// SelectRolePermissionByUserId 按用户ID查角色权限标识（对应 Java SysRoleServiceImpl#selectRolePermissionByUserId）。
// 复用 selectRolesByUserId 结果，再对每个 roleKey 按逗号切分去重。
func (s *RoleService) SelectRolePermissionByUserId(ctx context.Context, userID int64) ([]string, error) {
	roles, err := repository.NewRoleRepository(database.DB()).SelectRolesByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return splitRoleKeys(roles), nil
}

// splitRoleKeys 拆分角色 roleKey（可能形如 "a,b"）并去重，丢弃空白段。
// 对应 Java StringUtils.splitList(perm.getRoleKey().trim())。
func splitRoleKeys(roles []*model.SysRole) []string {
	set := make(map[string]struct{})
	out := make([]string, 0, len(roles))
	for i := range roles {
		if roles[i] == nil {
			continue
		}
		key := strings.TrimSpace(roles[i].RoleKey)
		if key == "" {
			continue
		}
		for _, part := range strings.Split(key, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := set[part]; ok {
				continue
			}
			set[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}
