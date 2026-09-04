package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

var ErrRoleNotFound = errors.New("repository: 角色不存在")

// RoleRepository sys_role 数据访问。
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 构造角色 repository。
func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// SelectRolesByUserId 按用户ID查其关联角色。
// 经 sys_user_role 关联，sys_role 的逻辑删除由实体自动过滤；不按角色状态过滤。
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

// ExistsRoleMenuByMenuID 判断菜单是否已分配给任何角色。
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

// DeleteRoleMenuByMenuIDs 按菜单主键批量清理角色-菜单关联。
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

// SelectPageList 按条件分页查角色。
func (r *RoleRepository) SelectPageList(ctx context.Context, q bo.SysRoleQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysRole], error) {

	db := applyRoleQuery(r.db.WithContext(ctx).Model(&model.SysRole{}), q)
	// 仅在调用方未指定排序时兜底。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("role_sort").Order("create_time")
	}

	var rows []*model.SysRole
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查角色，供导出与下拉选择等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyRoleQuery，保证两种路径的过滤条件永不漂移。
func (r *RoleRepository) SelectList(ctx context.Context, q bo.SysRoleQueryBo,
	limit int) ([]*model.SysRole, error) {

	db := applyRoleQuery(r.db.WithContext(ctx).Model(&model.SysRole{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定排序保证输出顺序稳定。
	db = db.Order("role_sort").Order("create_time")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysRole
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询角色列表失败: %w", err)
	}
	return rows, nil
}

// SelectByIDs 按主键批量查，返回实际命中的行（供删除前校验取角色名用）。
// 缺失主键静默跳过，由调用方比对数量。
func (r *RoleRepository) SelectByIDs(ctx context.Context,
	ids []int64) ([]*model.SysRole, error) {

	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysRole
	if err := r.db.WithContext(ctx).Where("role_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询角色 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectNormalByIDs 查启用状态的角色，ids 非空时按主键过滤
// （ids 为空即不加 IN 条件，退化成查全部启用角色）。
func (r *RoleRepository) SelectNormalByIDs(ctx context.Context,
	ids []int64) ([]*model.SysRole, error) {

	db := r.db.WithContext(ctx).Model(&model.SysRole{}).Where("status = ?", "0")
	if len(ids) > 0 {
		db = db.Where("role_id IN ?", ids)
	}
	// 固定按 role_sort 升序：下拉框选项顺序不该随 MySQL 返回顺序漂移。
	db = db.Order("role_sort")

	var rows []*model.SysRole
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询角色选择框列表失败: %w", err)
	}
	return rows, nil
}

// Insert 插入一条角色。
// role_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *RoleRepository) Insert(ctx context.Context, role *model.SysRole) error {
	if role == nil {
		return fmt.Errorf("repository: 角色为空")
	}
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return fmt.Errorf("repository: 插入角色失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *RoleRepository) UpdateByID(ctx context.Context, roleID int64,
	columns map[string]any) (int64, error) {

	if roleID <= 0 {
		return 0, fmt.Errorf("repository: 角色主键无效: %d", roleID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 角色更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysRole{}).
		Where("role_id = ?", roleID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新角色 id=%d 失败: %w", roleID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量逻辑删除，返回受影响行数。
// SysRole 嵌了 LogicDelete，Delete 会被改写成 UPDATE ... SET del_flag = '1'。
func (r *RoleRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 角色主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("role_id IN ?", ids).
		Delete(&model.SysRole{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除角色 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByRoleName 判断角色名称是否已被占用，excludeID > 0 时排除该主键
// （供修改场景排除自身）。
func (r *RoleRepository) ExistsByRoleName(ctx context.Context, roleName string,
	excludeID int64) (bool, error) {

	if roleName == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysRole{}).Where("role_name = ?", roleName)
	if excludeID > 0 {
		db = db.Where("role_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验角色名称 %q 失败: %w", roleName, err)
	}
	return count > 0, nil
}

// ExistsByRoleKey 判断角色权限字符是否已被占用，excludeID > 0 时排除该主键
// （供修改场景排除自身）。
func (r *RoleRepository) ExistsByRoleKey(ctx context.Context, roleKey string,
	excludeID int64) (bool, error) {

	if roleKey == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysRole{}).Where("role_key = ?", roleKey)
	if excludeID > 0 {
		db = db.Where("role_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验角色权限 %q 失败: %w", roleKey, err)
	}
	return count > 0, nil
}

// CountByRoleIDs 按主键集合统计存在的角色数。
//
// 数据权限的部门/角色隔离条件尚未落地：超管在 service 层短路，非超管
// 传入的 roleIds 都是自身可见的（前端只展示有权限的），count 不等只能说明
// 主键不存在。等数据权限落地后给这次 count 挂上过滤即可，调用点不必改。
func (r *RoleRepository) CountByRoleIDs(ctx context.Context, roleIDs []int64) (int64, error) {
	if len(roleIDs) == 0 {
		return 0, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SysRole{}).
		Where("role_id IN ?", roleIDs).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("repository: 统计角色 %v 失败: %w", roleIDs, err)
	}
	return count, nil
}

// applyRoleQuery 应用角色查询条件：
// 名称/权限字符走 LIKE，状态走 =，主键走 =，创建时间走闭区间。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyRoleQuery(db *gorm.DB, q bo.SysRoleQueryBo) *gorm.DB {
	if q.RoleID > 0 { // 0 不筛
		db = db.Where("role_id = ?", q.RoleID)
	}
	if q.RoleName != "" { // 空串不筛
		db = db.Where("role_name LIKE ?", "%"+escapeLike(q.RoleName)+"%")
	}
	if q.RoleKey != "" {
		db = db.Where("role_key LIKE ?", "%"+escapeLike(q.RoleKey)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 两端须同时给出：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	return db
}

// DeleteRoleMenuByRoleIDs 清理给定角色的菜单关联。sys_role_menu 无 del_flag，物理删除。
func (r *RoleRepository) DeleteRoleMenuByRoleIDs(ctx context.Context,
	roleIDs []int64) (int64, error) {

	if len(roleIDs) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Where("role_id IN ?", roleIDs).
		Delete(&model.SysRoleMenu{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理角色 %v 的菜单关联失败: %w", roleIDs, res.Error)
	}
	return res.RowsAffected, nil
}

// InsertRoleMenuBatch 批量写入角色-菜单关联。
// 空 list 直接返回，不报错——新增角色不勾任何菜单是合法用法。
func (r *RoleRepository) InsertRoleMenuBatch(ctx context.Context,
	list []model.SysRoleMenu) error {

	if len(list) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(list, len(list)).Error; err != nil {
		return fmt.Errorf("repository: 批量插入角色菜单关联失败: %w", err)
	}
	return nil
}

// DeleteRoleDeptByRoleIDs 清理给定角色的部门（数据权限）关联。sys_role_dept 无 del_flag，物理删除。
func (r *RoleRepository) DeleteRoleDeptByRoleIDs(ctx context.Context,
	roleIDs []int64) (int64, error) {

	if len(roleIDs) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Where("role_id IN ?", roleIDs).
		Delete(&model.SysRoleDept{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理角色 %v 的部门关联失败: %w", roleIDs, res.Error)
	}
	return res.RowsAffected, nil
}

// InsertRoleDeptBatch 批量写入角色-部门（数据权限）关联。
// 空 list 直接返回：dataScope 非「自定义」的角色本就没有部门勾选。
func (r *RoleRepository) InsertRoleDeptBatch(ctx context.Context,
	list []model.SysRoleDept) error {

	if len(list) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(list, len(list)).Error; err != nil {
		return fmt.Errorf("repository: 批量插入角色部门关联失败: %w", err)
	}
	return nil
}

// SelectDeptIDsByRoleID 按角色取其勾选的部门主键。
//
// 联 sys_role_dept + sys_role（要求角色状态正常），取部门后按 deptCheckStrictly
// 裁剪：严格模式下只留叶子勾选项——父级被勾选时其子项本就隐含勾选，回传父级会让
// 前端树把整棵子树渲染成已勾选不可逆，与菜单树同一套规则。非严格模式回传全部。
func (r *RoleRepository) SelectDeptIDsByRoleID(ctx context.Context,
	roleID int64, deptCheckStrictly bool) ([]int64, error) {

	if roleID <= 0 {
		return nil, nil
	}

	// DISTINCT 取部门 id + parent_id：parent_id 用于严格模式下识别"作为父级"的节点。
	var rows []struct {
		DeptID   int64 `gorm:"column:dept_id"`
		ParentID int64 `gorm:"column:parent_id"`
	}
	err := r.db.WithContext(ctx).
		Table("sys_dept AS d").
		Select("DISTINCT d.dept_id, d.parent_id").
		Joins("JOIN sys_role_dept srd ON srd.dept_id = d.dept_id").
		Joins("JOIN sys_role sr ON sr.role_id = srd.role_id").
		Where("srd.role_id = ?", roleID).
		Where("sr.status = ?", "0").
		Order("d.parent_id").
		Order("d.order_num").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询角色 %d 的部门关联失败: %w", roleID, err)
	}

	parentSet := map[int64]struct{}{}
	if deptCheckStrictly {
		for i := range rows {
			parentSet[rows[i].ParentID] = struct{}{}
		}
	}

	out := make([]int64, 0, len(rows))
	for i := range rows {
		if deptCheckStrictly {
			// 作为某节点父级的部门被排除，只留叶子勾选项。
			if _, ok := parentSet[rows[i].DeptID]; ok {
				continue
			}
		}
		out = append(out, rows[i].DeptID)
	}
	return out, nil
}

// SelectUserIDsByRoleID 取已分配该角色的用户ID集合。
// 供"未分配用户列表"排除已分配用户、以及角色变更时踢在线用户用。
func (r *RoleRepository) SelectUserIDsByRoleID(ctx context.Context,
	roleID int64) ([]int64, error) {

	if roleID <= 0 {
		return nil, nil
	}

	var ids []int64
	if err := r.db.WithContext(ctx).Model(&model.SysUserRole{}).
		Where("role_id = ?", roleID).
		Pluck("user_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询角色 %d 的用户失败: %w", roleID, err)
	}
	return ids, nil
}

// CountUserRoleByRoleID 统计已分配该角色的用户数。
// 角色被引用即不可删/不可禁用。
func (r *RoleRepository) CountUserRoleByRoleID(ctx context.Context, roleID int64) (int64, error) {
	if roleID <= 0 {
		return 0, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SysUserRole{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("repository: 统计角色 %d 的用户数失败: %w", roleID, err)
	}
	return count, nil
}

// DeleteUserRole 取消单个用户的角色授权。
// sys_user_role 无 del_flag，物理删除。
func (r *RoleRepository) DeleteUserRole(ctx context.Context,
	roleID, userID int64) (int64, error) {

	if roleID <= 0 || userID <= 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Where("user_id = ?", userID).
		Delete(&model.SysUserRole{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 取消用户 %d 的角色 %d 授权失败: %w",
			userID, roleID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteUserRolesByRoleID 批量取消角色下的用户授权。
func (r *RoleRepository) DeleteUserRolesByRoleID(ctx context.Context,
	roleID int64, userIDs []int64) (int64, error) {

	if roleID <= 0 || len(userIDs) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Where("user_id IN ?", userIDs).
		Delete(&model.SysUserRole{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 批量取消角色 %d 的用户授权失败: %w", roleID, res.Error)
	}
	return res.RowsAffected, nil
}

// InsertUserRoles 批量给角色追加用户授权。
// 空 userIds 直接返回：不勾任何用户是合法用法。
func (r *RoleRepository) InsertUserRoles(ctx context.Context, roleID int64,
	userIDs []int64) error {

	if roleID <= 0 || len(userIDs) == 0 {
		return nil
	}

	list := make([]model.SysUserRole, 0, len(userIDs))
	for _, uid := range userIDs {
		list = append(list, model.SysUserRole{UserID: uid, RoleID: roleID})
	}
	if err := r.db.WithContext(ctx).CreateInBatches(list, len(list)).Error; err != nil {
		return fmt.Errorf("repository: 批量插入角色 %d 的用户授权失败: %w", roleID, err)
	}
	return nil
}
