package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/snowflake"
	"ruoyi-go-vue-plus/pkg/strutil"
	"ruoyi-go-vue-plus/pkg/tree"
)

var ErrMenuNotFound = errors.New("service: 菜单不存在")

var ErrMenuNameExists = errors.New("service: 菜单名称已存在")

var ErrMenuParentIsSelf = errors.New("service: 上级菜单不能选择自己")

var ErrMenuFrameNeedHTTP = errors.New("service: 外链地址必须以http(s)://开头")

var ErrMenuRouteConflict = errors.New("service: 路由名称或地址已存在")

// MenuService 菜单业务逻辑。
type MenuService struct{}

var MenuSvcApp = new(MenuService)

// SelectMenuPermsByUserId 按用户ID查菜单权限标识。
func (s *MenuService) SelectMenuPermsByUserId(ctx context.Context, userID int64) ([]string, error) {
	return repository.NewMenuRepository(database.DB()).SelectMenuPermsByUserId(ctx, userID)
}

// SelectMenuPermsByRoleIds 按角色ID集合查权限。
// 返回 map[roleId]perms，每个角色的 perms 去重且保序。
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

// SelectMenuTreeByUserId 按用户ID查菜单树。
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

// BuildMenus 构建前端路由。
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

// QueryList 按条件查菜单列表。
// 超管拿全量，其余用户按其角色关联的菜单过滤。
func (s *MenuService) QueryList(ctx context.Context, q bo.SysMenuQueryBo,
	userID int64) ([]*vo.SysMenuVo, error) {

	repo := repository.NewMenuRepository(database.DB())

	var (
		menus []*model.SysMenu
		err   error
	)
	if loginhelper.IsSuperAdmin(userID) {
		menus, err = repo.SelectList(ctx, q)
	} else {
		menus, err = repo.SelectListByUserId(ctx, q, userID)
	}
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysMenuVoList(menus), nil
}

// QueryByID 按主键查菜单，不存在时返回 ErrMenuNotFound。
func (s *MenuService) QueryByID(ctx context.Context, menuID int64) (*vo.SysMenuVo, error) {
	menu, err := repository.NewMenuRepository(database.DB()).SelectByID(ctx, menuID)
	if err != nil {
		if errors.Is(err, repository.ErrMenuNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysMenuVo(menu), nil
}

// BuildMenuTreeSelect 把菜单列表构造成前端下拉树。
//
// 走 tree.Build 而非 BuildInPlace：树节点是独立的 Tree[int64]（含 label/weight 与
// 四个 Extra 键），不是 SysMenuVo 本身。根节点固定取 0——菜单列表按 parent_id 升序，
// 首元素必是顶级菜单。
func (s *MenuService) BuildMenuTreeSelect(menus []*vo.SysMenuVo) []*tree.Tree[int64] {
	return tree.Build(menus, constant.ConstantTopParentID,
		func(m *vo.SysMenuVo, node *tree.Tree[int64]) {
			node.ID = m.MenuID
			node.ParentID = m.ParentID
			node.Name = m.MenuName
			node.Weight = m.OrderNum
			node.SetExtra("menuType", m.MenuType)
			node.SetExtra("icon", m.Icon)
			node.SetExtra("visible", m.Visible)
			node.SetExtra("status", m.Status)
		})
}

// SelectMenuIDsByRoleID 按角色查其选中的菜单主键。
// 角色不存在时返回 ErrRoleNotFound。
func (s *MenuService) SelectMenuIDsByRoleID(ctx context.Context, roleID int64) ([]int64, error) {
	role, err := repository.NewRoleRepository(database.DB()).SelectByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return repository.NewMenuRepository(database.DB()).
		SelectMenuIDsByRoleID(ctx, roleID, role.MenuCheckStrictly)
}

// CheckMenuNameUnique 校验同一上级下的菜单名称是否可用（唯一即 true）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *MenuService) CheckMenuNameUnique(ctx context.Context, menuName string,
	parentID, excludeID int64) (bool, error) {

	exists, err := repository.NewMenuRepository(database.DB()).
		ExistsByMenuName(ctx, menuName, parentID, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertMenu 新增菜单。插入成功后回填 b.MenuID。
func (s *MenuService) InsertMenu(ctx context.Context, b *bo.SysMenuBo) error {
	if b == nil {
		return errors.New("service: 菜单入参为空")
	}
	if err := s.validateMenu(ctx, b, 0); err != nil {
		return err
	}

	add := bo.Conv.ConvertToSysMenu(b)
	add.MenuID = snowflake.Next() // menu_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewMenuRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.MenuID = add.MenuID
	return nil
}

// UpdateMenu 按主键修改菜单。
func (s *MenuService) UpdateMenu(ctx context.Context, b *bo.SysMenuBo) error {
	if b == nil {
		return errors.New("service: 菜单入参为空")
	}
	if b.MenuID <= 0 {
		return errors.New("service: 菜单主键不能为空")
	}
	if err := s.validateMenu(ctx, b, b.MenuID); err != nil {
		return err
	}

	repo := repository.NewMenuRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"菜单不存在"。
	if _, err := repo.SelectByID(ctx, b.MenuID); err != nil {
		if errors.Is(err, repository.ErrMenuNotFound) {
			return ErrMenuNotFound
		}
		return err
	}

	_, err := repo.UpdateByID(ctx, b.MenuID, buildMenuUpdateColumns(b))
	return err
}

// validateMenu 新增/修改共用的四道校验：
// 名称重复 → 外链地址 → 上级是自己（仅修改）→ 路由冲突。
// 同时触发多条时，前端看到的提示保持稳定。
//
// excludeID 为 0 表示新增场景（无自身可排除）。
func (s *MenuService) validateMenu(ctx context.Context, b *bo.SysMenuBo, excludeID int64) error {
	unique, err := s.CheckMenuNameUnique(ctx, b.MenuName, b.ParentID, excludeID)
	if err != nil {
		return err
	}
	if !unique {
		return ErrMenuNameExists
	}
	// 外链菜单的 path 直接给浏览器跳转用，不是 http 地址会拼出打不开的链接。
	if b.IsFrame == constant.Yes && !strutil.IsHTTP(b.Path) {
		return ErrMenuFrameNeedHTTP
	}
	// 认自己作父会让菜单树自指，构树时直接成环。新增时 excludeID 为 0，无需校验。
	if excludeID > 0 && b.ParentID == b.MenuID {
		return ErrMenuParentIsSelf
	}
	ok, err := s.checkRouteConfigUnique(ctx, b)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMenuRouteConflict
	}
	return nil
}

// checkRouteConfigUnique 校验路由组合是否唯一。
// 按钮无路由，直接放行。
func (s *MenuService) checkRouteConfigUnique(ctx context.Context, b *bo.SysMenuBo) (bool, error) {
	if b.MenuType == constant.MenuTypeButton {
		return true, nil
	}

	// routeName 缺省时以 path 代之，两者都参与候选集匹配。
	routeName := menuRouteName(b.Path, b.MenuType, b.ParentID)
	candidates, err := repository.NewMenuRepository(database.DB()).
		SelectRouteConflictCandidates(ctx, b.Path, routeName)
	if err != nil {
		return false, err
	}
	return routeConfigUnique(b, routeName, candidates), nil
}

// routeConfigUnique 在候选集里逐条判定路由冲突。
//
// 与 checkRouteConfigUnique 分开是为了可测：三条规则的边界（大小写不敏感、
// 跨级同名放行、按钮不参与）是本模块最容易写错的地方，抽成纯函数才能不连库覆盖。
func routeConfigUnique(b *bo.SysMenuBo, routeName string, candidates []*model.SysMenu) bool {
	// menuID 为 0（新增）时不会等于任何库中主键。
	for _, db := range candidates {
		if db == nil || db.MenuID == b.MenuID {
			continue
		}
		dbRouteName := menuRouteName(db.Path, db.MenuType, db.ParentID)
		switch {
		// 同级下路由路径必须唯一，否则前端同一父路由下会注册两条同 path 子路由。
		case strings.EqualFold(b.Path, db.Path) && b.ParentID == db.ParentID:
			log.Printf("[menu] 同级路由冲突: 同级下已存在相同路由路径 %q，冲突菜单：%s",
				db.Path, db.MenuName)
			return false
		// 根目录下的路径是一级路由，全局唯一。
		//
		// 这条分支实际不可达：两者 parentID 同为 0 时第一条已命中。
		case strings.EqualFold(b.Path, db.Path) &&
			b.ParentID == constant.ConstantTopParentID &&
			db.ParentID == constant.ConstantTopParentID:
			log.Printf("[menu] 根目录路由冲突: 根目录下路由 %q 必须唯一，已被菜单 %q 占用",
				b.Path, db.MenuName)
			return false
		// 路由 name 是 vue-router 的全局键，同类型下重名会让后注册的覆盖前者。
		case strings.EqualFold(routeName, dbRouteName) && b.MenuType == db.MenuType:
			log.Printf("[menu] 路由名称冲突: 路由名称 %q 需全局唯一，已被菜单 %q 使用",
				routeName, db.MenuName)
			return false
		}
	}
	return true
}

// menuRouteName 取菜单的路由名称，空则以 path 兜底。
//
// 复用 model.SysMenu 的 RouteName 而不另写一份：一级非外链菜单的 routeName 恒为空，
// 这条规则只落在实体方法里，重写必然漂移。
func menuRouteName(path, menuType string, parentID int64) string {
	m := &model.SysMenu{Path: path, MenuType: menuType, ParentID: parentID,
		IsFrame: constant.No}
	if rn := m.RouteName(); rn != "" {
		return rn
	}
	return path
}

// buildMenuUpdateColumns 组装修改菜单的更新列。
func buildMenuUpdateColumns(b *bo.SysMenuBo) map[string]any {
	columns := map[string]any{
		"parent_id": b.ParentID,
		"menu_name": b.MenuName,
		"order_num": b.OrderNum,
		"path":      b.Path,
		"menu_type": b.MenuType,
		// 以下五者一律写入，让前端能把它们清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		// component 尤其关键：目录改成按钮时必须能清掉组件路径。
		"component":   b.Component,
		"query_param": b.QueryParam,
		"perms":       b.Perms,
		"active_menu": b.ActiveMenu,
		"remark":      b.Remark,
	}
	// icon/ext 与三个控制字段缺省即视为不改：漏传不该把线上的 'Y'/'0' 刷成空串，
	// 那会让 char(1) 列既不是 Y 也不是 N。
	if b.Icon != "" {
		columns["icon"] = b.Icon
	}
	if b.Ext != "" {
		columns["ext"] = b.Ext
	}
	if b.IsFrame != "" {
		columns["is_frame"] = b.IsFrame
	}
	if b.IsCache != "" {
		columns["is_cache"] = b.IsCache
	}
	if b.Visible != "" {
		columns["visible"] = b.Visible
	}
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// DeleteMenuByID 删除单个菜单。
//
// 两道拦截留在 service 而非 handler：它们是数据完整性约束而非 HTTP 关注点，
// 放这里才能挡住将来其它调用路径。
func (s *MenuService) DeleteMenuByID(ctx context.Context, menuID int64) error {
	if menuID <= 0 {
		return errors.New("service: 菜单主键不能为空")
	}

	repo := repository.NewMenuRepository(database.DB())
	hasChild, err := repo.ExistsByParentIDs(ctx, []int64{menuID})
	if err != nil {
		return err
	}
	if hasChild {
		return errs.New(response.CodeWarn, "存在子菜单,不允许删除", "")
	}

	// 已分配给角色的菜单直接删会留下悬空的 sys_role_menu 行；单删不做级联清理，
	// 要连带清理请走 DeleteMenuByIDs。
	assigned, err := repository.NewRoleRepository(database.DB()).
		ExistsRoleMenuByMenuID(ctx, menuID)
	if err != nil {
		return err
	}
	if assigned {
		return errs.New(response.CodeWarn, "菜单已分配,不允许删除", "")
	}

	affected, err := repo.DeleteByIDs(ctx, []int64{menuID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMenuNotFound
	}
	return nil
}

// DeleteMenuByIDs 批量级联删除菜单。
//
// 与单删的差别：不校验"已分配角色"，而是连带清理 sys_role_menu——级联删除的语义
// 就是"连同授权一起去掉"。子菜单校验仍在，且排除本批自身（父子同批提交是合法用法）。
func (s *MenuService) DeleteMenuByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 菜单主键不能为空")
	}

	repo := repository.NewMenuRepository(database.DB())
	hasChild, err := repo.ExistsByParentIDs(ctx, ids)
	if err != nil {
		return err
	}
	if hasChild {
		return errs.New(response.CodeWarn, "存在子菜单,不允许删除", "")
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 先删菜单再清关联：这里没有事务包裹，反序一旦中途失败会留下"授权已丢、菜单还在"
	// 的状态——那比残留几行 sys_role_menu 更难修，后者重新走一次级联删即可收敛。
	if _, err := repository.NewRoleRepository(database.DB()).
		DeleteRoleMenuByMenuIDs(ctx, ids); err != nil {
		return err
	}
	return nil
}
