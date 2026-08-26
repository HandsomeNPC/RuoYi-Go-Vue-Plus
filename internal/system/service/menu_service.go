package service

import (
	"context"

	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// MenuService 菜单业务逻辑。
type MenuService struct{}

// MenuSvcApp 包级实例。
var MenuSvcApp = new(MenuService)

// SelectMenuPermsByUserId 按用户ID查菜单权限标识（对应 Java SysMenuServiceImpl#selectMenuPermsByUserId）。
func (s *MenuService) SelectMenuPermsByUserId(ctx context.Context, userID int64) ([]string, error) {
	return repository.NewMenuRepository(database.DB()).SelectMenuPermsByUserId(ctx, userID)
}

// SelectMenuPermsByRoleIds 按角色ID集合查权限（对应 Java SysMenuServiceImpl#selectMenuPermsByRoleIds）。
// 返回 map[roleId]perms，每个角色的 perms 去重且保序（对应 Java LinkedHashMap + LinkedHashSet）。
func (s *MenuService) SelectMenuPermsByRoleIds(ctx context.Context, roleIDs []int64) (map[int64][]string, error) {
	rows, err := repository.NewMenuRepository(database.DB()).SelectMenuPermsByRoleIds(ctx, roleIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[int64][]string)
	seen := make(map[int64]map[string]struct{}, len(rows))
	for i := range rows {
		roleID := rows[i].RoleID
		perms := rows[i].Perms
		if perms == "" {
			continue
		}
		dedup, ok := seen[roleID]
		if !ok {
			dedup = make(map[string]struct{})
			seen[roleID] = dedup
		}
		if _, dup := dedup[perms]; dup {
			continue
		}
		dedup[perms] = struct{}{}
		out[roleID] = append(out[roleID], perms)
	}
	return out, nil
}
