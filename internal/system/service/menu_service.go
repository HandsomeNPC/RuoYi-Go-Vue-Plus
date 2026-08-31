package service

import (
	"context"
	"strconv"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/strutil"
	"ruoyi-go-vue-plus/pkg/tree"
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

// SelectMenuTreeByUserId 按用户ID查菜单树（对应 Java SysMenuServiceImpl#selectMenuTreeByUserId）。
// 超管拿全量目录+菜单，其余用户按角色过滤。扁平结果就地组装成树：走 tree.BuildInPlace
// 而非 tree.Build，顺序已由 SQL 的 ORDER BY parent_id, order_num 定好且无需再排，节点是
// SysMenu 实体本身而非独立树节点。
func (s *MenuService) SelectMenuTreeByUserId(ctx context.Context, userID int64) ([]*model.SysMenu, error) {
	repo := repository.NewMenuRepository(database.DB())

	var (
		menus []*model.SysMenu
		err   error
	)
	if loginhelper.IsSuperAdmin(userID) {
		menus, err = repo.SelectMenuTreeAll(ctx)
	} else {
		menus, err = repo.SelectMenuTreeByUserId(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	return tree.BuildInPlace(menus, constant.ConstantTopParentID,
		func(m *model.SysMenu) int64 { return m.MenuID },
		func(m *model.SysMenu) int64 { return m.ParentID },
		func(m *model.SysMenu, children []*model.SysMenu) { m.Children = children }), nil
}

// BuildMenus 构建前端路由（对应 Java SysMenuServiceImpl#buildMenus）。
// 路由 name 规则：path 首字母大写 + menuId。
func (s *MenuService) BuildMenus(menus []*model.SysMenu) []*vo.RouterVo {
	routers := make([]*vo.RouterVo, 0, len(menus))
	for _, menu := range menus {
		router := &vo.RouterVo{
			Hidden:    menu.Visible == constant.StatusDisable,
			Name:      menu.RouteName() + strconv.FormatInt(menu.MenuID, 10),
			Path:      menu.RouterPath(),
			Component: menu.ComponentInfo(),
			Query:     menu.QueryParam,
			Ext:       menu.Ext,
			Meta: vo.NewMetaVo(menu.MenuName, menu.Icon,
				menu.IsCache == constant.No, menu.Path, menu.ActiveMenu),
		}

		switch {
		case len(menu.Children) > 0 && menu.MenuType == constant.MenuTypeDir:
			alwaysShow := true
			router.AlwaysShow = &alwaysShow
			// noRedirect 让面包屑里的该级不可点击。
			router.Redirect = "noRedirect"
			router.Children = s.BuildMenus(menu.Children)

		case menu.IsMenuFrame():
			// 一级菜单在前端要挂到根路由(path="/")下，自身 meta 交给这个子路由承载。
			router.Meta = nil
			router.Children = []*vo.RouterVo{{
				Path:      menu.Path,
				Component: menu.Component,
				Name:      strutil.Capitalize(menu.Path) + strconv.FormatInt(menu.MenuID, 10),
				Meta: vo.NewMetaVo(menu.MenuName, menu.Icon,
					menu.IsCache == constant.No, menu.Path, menu.ActiveMenu),
				Query: menu.QueryParam,
				Ext:   menu.Ext,
			}}

		case menu.ParentID == constant.ConstantTopParentID && menu.IsInnerLink():
			router.Meta = vo.NewMetaVo(menu.MenuName, menu.Icon, false, "", "")
			router.Path = "/"
			routerPath := model.InnerLinkReplaceEach(menu.Path)
			router.Children = []*vo.RouterVo{{
				Path:      routerPath,
				Component: constant.ComponentInnerLink,
				Name:      strutil.Capitalize(routerPath) + strconv.FormatInt(menu.MenuID, 10),
				// 内链把原始 http 地址塞进 meta.link 供 iframe 加载。
				Meta: vo.NewMetaVo(menu.MenuName, menu.Icon, false, menu.Path, ""),
				Ext:  menu.Ext,
			}}
		}

		routers = append(routers, router)
	}
	return routers
}
