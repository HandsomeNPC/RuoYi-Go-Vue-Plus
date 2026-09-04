package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/pkg/constant"
)

var ErrMenuNotFound = errors.New("repository: 菜单不存在")

// MenuRepository sys_menu 数据访问。
type MenuRepository struct {
	db *gorm.DB
}

// NewMenuRepository 构造菜单 repository。
func NewMenuRepository(db *gorm.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

// RoleMenuPerm 角色-菜单权限投影行。
// 非表实体，仅用于联表查询结果承载。
type RoleMenuPerm struct {
	RoleID int64  `gorm:"column:role_id"`
	Perms  string `gorm:"column:perms"`
}

// SelectMenuPermsByUserId 按用户ID查菜单权限标识。
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

// SelectMenuPermsByRoleIds 按角色ID集合批量查权限。
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

// SelectMenuTreeAll 查全部正常状态的目录和菜单。
// 排序 parent_id, order_num 是构树前提：service 依赖同级节点已按 order_num 有序。
func (r *MenuRepository) SelectMenuTreeAll(ctx context.Context) ([]*model.SysMenu, error) {
	var menus []*model.SysMenu
	err := r.db.WithContext(ctx).
		Where("menu_type IN ?", []string{constant.MenuTypeDir, constant.MenuTypeMenu}).
		Where("status = ?", constant.StatusNormal).
		Order("parent_id").
		Order("order_num").
		Find(&menus).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询全部菜单树失败: %w", err)
	}
	return menus, nil
}

// SelectMenuTreeByUserId 按用户ID查其可见的目录和菜单。
func (r *MenuRepository) SelectMenuTreeByUserId(ctx context.Context, userID int64) ([]*model.SysMenu, error) {
	if userID <= 0 {
		return nil, nil
	}

	var menus []*model.SysMenu
	err := r.db.WithContext(ctx).
		Model(&model.SysMenu{}).
		Distinct("sys_menu.*").
		Joins("JOIN sys_role_menu srm ON srm.menu_id = sys_menu.menu_id").
		Joins("JOIN sys_user_role sur ON sur.role_id = srm.role_id").
		Joins("JOIN sys_role sr ON sr.role_id = srm.role_id").
		Where("sur.user_id = ?", userID).
		Where("sr.status = ?", constant.StatusNormal).
		Where("sr.del_flag = ?", constant.StatusNormal).
		Where("sys_menu.menu_type IN ?", []string{constant.MenuTypeDir, constant.MenuTypeMenu}).
		Where("sys_menu.status = ?", constant.StatusNormal).
		Order("sys_menu.parent_id").
		Order("sys_menu.order_num").
		Find(&menus).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 菜单树失败: %w", userID, err)
	}
	return menus, nil
}

// SelectByID 按主键查菜单，不存在时返回 ErrMenuNotFound。
func (r *MenuRepository) SelectByID(ctx context.Context, menuID int64) (*model.SysMenu, error) {
	if menuID <= 0 {
		return nil, ErrMenuNotFound
	}

	var menu model.SysMenu
	err := r.db.WithContext(ctx).
		Where("menu_id = ?", menuID).
		First(&menu).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, fmt.Errorf("repository: 查询菜单 id=%d 失败: %w", menuID, err)
	}
	return &menu, nil
}

// SelectList 按条件查全部菜单。
// 菜单总量有限（前端按树整体渲染、无分页），故不分页也不限行数。
func (r *MenuRepository) SelectList(ctx context.Context, q bo.SysMenuQueryBo) ([]*model.SysMenu, error) {
	db := applyMenuQuery(r.db.WithContext(ctx).Model(&model.SysMenu{}), q)
	// parent_id, order_num 是构树前提：service 依赖同级节点已按 order_num 有序。
	db = db.Order("parent_id").Order("order_num")

	var menus []*model.SysMenu
	if err := db.Find(&menus).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询菜单列表失败: %w", err)
	}
	return menus, nil
}

// SelectListByUserId 按条件查用户可见的菜单。
// 经 sys_role_menu / sys_user_role 关联，仅取正常状态角色下的菜单。
func (r *MenuRepository) SelectListByUserId(ctx context.Context, q bo.SysMenuQueryBo,
	userID int64) ([]*model.SysMenu, error) {

	if userID <= 0 {
		return nil, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysMenu{}).
		Distinct("sys_menu.*").
		Joins("LEFT JOIN sys_role_menu srm ON srm.menu_id = sys_menu.menu_id").
		Joins("LEFT JOIN sys_user_role sur ON sur.role_id = srm.role_id").
		Joins("LEFT JOIN sys_role sr ON sr.role_id = srm.role_id").
		Where("sur.user_id = ?", userID).
		Where("sr.status = ?", constant.StatusNormal)
	// 列名带 sys_menu. 前缀：三张联表都有 status，不限定会撞上 sr.status 而歧义。
	db = applyMenuQueryWithAlias(db, q, "sys_menu.")
	db = db.Order("sys_menu.parent_id").Order("sys_menu.order_num")

	var menus []*model.SysMenu
	if err := db.Find(&menus).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 菜单列表失败: %w", userID, err)
	}
	return menus, nil
}

// applyMenuQuery 应用菜单查询条件（单表场景，无需列名前缀）。
func applyMenuQuery(db *gorm.DB, q bo.SysMenuQueryBo) *gorm.DB {
	return applyMenuQueryWithAlias(db, q, "")
}

// applyMenuQueryWithAlias 应用菜单查询条件：
// 名称走 LIKE，其余走 =；空值一概不筛。
//
// prefix 是列名前缀（联表时传 "sys_menu."）：超管与普通用户两条路径必须共用本函数，
// 否则过滤逻辑改一处漏一处。
func applyMenuQueryWithAlias(db *gorm.DB, q bo.SysMenuQueryBo, prefix string) *gorm.DB {
	if q.MenuName != "" {
		db = db.Where(prefix+"menu_name LIKE ?", "%"+escapeLike(q.MenuName)+"%")
	}
	if q.Visible != "" {
		db = db.Where(prefix+"visible = ?", q.Visible)
	}
	if q.Status != "" {
		db = db.Where(prefix+"status = ?", q.Status)
	}
	if q.MenuType != "" {
		db = db.Where(prefix+"menu_type = ?", q.MenuType)
	}
	if q.ParentID > 0 {
		db = db.Where(prefix+"parent_id = ?", q.ParentID)
	}
	return db
}

// SelectMenuIDsByRoleID 按角色查其选中的菜单主键。
//
// menuCheckStrictly 为 true 时剔除"同时又是别的选中项之父"的节点：前端树组件
// 在父子联动模式下，父节点勾选态由子节点推导，回传父节点会让它显示成全选。
func (r *MenuRepository) SelectMenuIDsByRoleID(ctx context.Context, roleID int64,
	menuCheckStrictly bool) ([]int64, error) {

	if roleID <= 0 {
		return nil, nil
	}

	// 只取构树判定所需的两列，不 SELECT *：这里的结果只用来算主键集合。
	// order_num 进 SELECT 是 DISTINCT + ORDER BY 的要求（排序列须在投影里），
	// 但不必落到结构体上。
	type menuIDRow struct {
		MenuID   int64 `gorm:"column:menu_id"`
		ParentID int64 `gorm:"column:parent_id"`
	}
	var rows []menuIDRow
	err := r.db.WithContext(ctx).
		Table("sys_menu AS m").
		Select("m.menu_id, m.parent_id, m.order_num").
		Distinct().
		Joins("LEFT JOIN sys_role_menu srm ON srm.menu_id = m.menu_id").
		Joins("LEFT JOIN sys_role sr ON sr.role_id = srm.role_id").
		Where("srm.role_id = ?", roleID).
		Where("sr.status = ?", constant.StatusNormal).
		Order("m.parent_id").
		Order("m.order_num").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询角色 %d 菜单失败: %w", roleID, err)
	}

	parentIDs := make(map[int64]struct{})
	if menuCheckStrictly {
		for i := range rows {
			parentIDs[rows[i].ParentID] = struct{}{}
		}
	}
	out := make([]int64, 0, len(rows))
	for i := range rows {
		if _, isParent := parentIDs[rows[i].MenuID]; isParent {
			continue
		}
		out = append(out, rows[i].MenuID)
	}
	return out, nil
}

// Insert 插入一条菜单。
// menu_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *MenuRepository) Insert(ctx context.Context, menu *model.SysMenu) error {
	if menu == nil {
		return fmt.Errorf("repository: 菜单为空")
	}
	if err := r.db.WithContext(ctx).Create(menu).Error; err != nil {
		return fmt.Errorf("repository: 插入菜单失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空权限标识/路由参数」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *MenuRepository) UpdateByID(ctx context.Context, menuID int64,
	columns map[string]any) (int64, error) {

	if menuID <= 0 {
		return 0, fmt.Errorf("repository: 菜单主键无效: %d", menuID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 菜单更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysMenu{}).
		Where("menu_id = ?", menuID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新菜单 id=%d 失败: %w", menuID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_menu 无 del_flag，这是物理删除。
func (r *MenuRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 菜单主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("menu_id IN ?", ids).
		Delete(&model.SysMenu{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除菜单 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByParentIDs 判断这批菜单之外是否还有以它们为父的菜单。
//
// 排除 ids 自身：级联删除时父子可能同批提交，此时子菜单会随父一起删掉，不该算"存在子菜单"。
// 传单个 id 时 NOT IN 自身恒真，退化成单参重载。
func (r *MenuRepository) ExistsByParentIDs(ctx context.Context, ids []int64) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysMenu{}).
		Where("parent_id IN ?", ids).
		Where("menu_id NOT IN ?", ids).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询菜单 %v 的子菜单失败: %w", ids, err)
	}
	return count > 0, nil
}

// ExistsByMenuName 判断同一上级下的菜单名称是否已被占用，excludeID > 0 时排除该主键
// （名称 + 父级两列共同判重，供修改场景排除自身）。
func (r *MenuRepository) ExistsByMenuName(ctx context.Context, menuName string,
	parentID, excludeID int64) (bool, error) {

	if menuName == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysMenu{}).
		Where("menu_name = ?", menuName).
		Where("parent_id = ?", parentID)
	if excludeID > 0 {
		db = db.Where("menu_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验菜单名称 %q 失败: %w", menuName, err)
	}
	return count > 0, nil
}

// SelectRouteConflictCandidates 查 path 或 route_name 命中给定两值的目录/菜单。
//
// 只捞候选、不在 SQL 里判冲突：冲突规则要比对 parent_id 与 menu_type 的多种组合，
// 放 service 里逐条判定比拼一个大 WHERE 可读得多。
func (r *MenuRepository) SelectRouteConflictCandidates(ctx context.Context,
	path, routeName string) ([]*model.SysMenu, error) {

	var menus []*model.SysMenu
	err := r.db.WithContext(ctx).Model(&model.SysMenu{}).
		Where("menu_type IN ?", []string{constant.MenuTypeDir, constant.MenuTypeMenu}).
		Where("path = ? OR path = ?", path, routeName).
		Find(&menus).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询路由冲突候选失败: %w", err)
	}
	return menus, nil
}
