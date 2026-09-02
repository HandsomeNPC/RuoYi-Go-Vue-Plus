package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// ErrRoleNotFound 角色不存在。
var ErrRoleNotFound = errors.New("service: 角色不存在")

// ErrRoleNameExists 角色名称已被占用。
var ErrRoleNameExists = errors.New("service: 角色名称已存在")

// ErrRoleKeyExists 角色权限字符已被占用。
var ErrRoleKeyExists = errors.New("service: 角色权限已存在")

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

// QueryPageList 按条件分页查角色（对应 Java selectPageRoleList）。
func (s *RoleService) QueryPageList(ctx context.Context, q bo.SysRoleQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysRoleVo], error) {

	res, err := repository.NewRoleRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysRoleVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysRoleVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查角色，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *RoleService) QueryList(ctx context.Context, q bo.SysRoleQueryBo,
	limit int) ([]*vo.SysRoleVo, error) {

	rows, err := repository.NewRoleRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysRoleVoList(rows), nil
}

// QueryByID 按主键查角色（对应 Java selectRoleById），不存在时返回 ErrRoleNotFound。
func (s *RoleService) QueryByID(ctx context.Context, roleID int64) (*vo.SysRoleVo, error) {
	role, err := repository.NewRoleRepository(database.DB()).SelectByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysRoleVo(role), nil
}

// SelectByIDs 取角色选择框列表（对应 Java selectRoleByIds）。
// ids 为空返回全部启用角色，非空按主键过滤且只取启用状态。
func (s *RoleService) SelectByIDs(ctx context.Context,
	ids []int64) ([]*vo.SysRoleVo, error) {

	rows, err := repository.NewRoleRepository(database.DB()).SelectNormalByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysRoleVoList(rows), nil
}

// SelectDeptIDsByRoleID 取角色的部门（数据权限）勾选集合（对应 Java
// selectDeptListByRoleId）。用于角色部门树回填 checkedKeys。
// 严格模式（DeptCheckStrictly）只回叶子勾选项——父级勾选时子项隐含，回传父级
// 会让前端树把整棵子树渲染成不可逆的全选。
func (s *RoleService) SelectDeptIDsByRoleID(ctx context.Context,
	roleID int64) ([]int64, error) {

	role, err := repository.NewRoleRepository(database.DB()).SelectByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return repository.NewRoleRepository(database.DB()).
		SelectDeptIDsByRoleID(ctx, roleID, role.DeptCheckStrictly)
}

// CheckRoleNameUnique 校验角色名称是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *RoleService) CheckRoleNameUnique(ctx context.Context, roleName string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewRoleRepository(database.DB()).
		ExistsByRoleName(ctx, roleName, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckRoleKeyUnique 校验角色权限字符是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *RoleService) CheckRoleKeyUnique(ctx context.Context, roleKey string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewRoleRepository(database.DB()).
		ExistsByRoleKey(ctx, roleKey, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckRoleAllowed 校验角色是否允许操作（对应 Java checkRoleAllowed）。
// 超管角色不可增删改，超管 roleKey 不可被借用或替换。失败时返回带文案的 ServiceError。
func (s *RoleService) CheckRoleAllowed(ctx context.Context, b *bo.SysRoleBo) error {
	if b == nil {
		return nil
	}
	if b.RoleID == constant.SuperAdminRoleID {
		return errs.New(0, "不允许操作超级管理员角色", "")
	}
	keys := []string{constant.SuperAdminRoleKey}
	if b.RoleID == 0 {
		// 新增不允许使用超管标识符。
		if equalsAny(b.RoleKey, keys) {
			return errs.New(0, "不允许使用系统内置管理员角色标识符!", "")
		}
		return nil
	}
	// 修改：标识符变化时校验新旧两端都不是超管。
	existing, err := repository.NewRoleRepository(database.DB()).SelectByID(ctx, b.RoleID)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		return err
	}
	if existing.RoleKey != b.RoleKey {
		if equalsAny(existing.RoleKey, keys) {
			return errs.New(0, "不允许修改系统内置管理员角色标识符!", "")
		}
		if equalsAny(b.RoleKey, keys) {
			return errs.New(0, "不允许使用系统内置管理员角色标识符!", "")
		}
	}
	return nil
}

// CheckRoleDataScope 校验当前用户对给定角色是否有数据权限（对应 Java checkRoleDataScope）。
//
// 超管短路放行；非超管按主键统计可见角色数，与传入数量不等即越权。
// Java 的 selectRoleCount 带 @DataPermission 注入隔离条件，Go 侧数据权限尚未落地，
// count 只按主键计——传入的 roleIds 都是前端可见的，不等只能说明主键不存在。
// 等数据权限落地后给 count 挂上过滤即可，调用点不必改。
func (s *RoleService) CheckRoleDataScope(ctx context.Context, userID, roleID int64) error {
	if roleID <= 0 {
		return nil
	}
	if loginhelper.IsSuperAdmin(userID) {
		return nil
	}
	count, err := repository.NewRoleRepository(database.DB()).
		CountByRoleIDs(ctx, []int64{roleID})
	if err != nil {
		return err
	}
	if count != 1 {
		return errs.New(0, "没有权限访问部分角色数据！", "")
	}
	return nil
}

// InsertRole 新增角色（对应 Java insertRole）。名称/权限重复或操作超管时返回对应错误。
// 插入主角色后回写 b.RoleID，再批量写菜单授权；角色-部门（数据权限）不在新增路径写，
// 由 UpdateRolePermission 单独承接——与 Java 一致。
func (s *RoleService) InsertRole(ctx context.Context, b *bo.SysRoleBo) error {
	if b == nil {
		return errors.New("service: 角色入参为空")
	}
	if err := s.CheckRoleAllowed(ctx, b); err != nil {
		return err
	}
	if unique, err := s.CheckRoleNameUnique(ctx, b.RoleName, 0); err != nil {
		return err
	} else if !unique {
		return ErrRoleNameExists
	}
	if unique, err := s.CheckRoleKeyUnique(ctx, b.RoleKey, 0); err != nil {
		return err
	} else if !unique {
		return ErrRoleKeyExists
	}

	repo := repository.NewRoleRepository(database.DB())
	add := bo.Conv.ConvertToSysRole(b)
	add.RoleID = snowflake.Next() // role_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。
	if err := repo.Insert(ctx, add); err != nil {
		return err
	}
	b.RoleID = add.RoleID
	return s.insertRoleMenu(ctx, repo, b.RoleID, b.MenuIDs)
}

// UpdateRoleBaseInfo 修改角色基础信息（对应 Java updateRoleBaseInfo）。
// 只更新 name/key/sort/status/remark，不触碰 data_scope/树联动（权限域，由
// UpdateRolePermission 承接），与 Java 注释「仅更新基础字段，避免影响权限分配」一致。
// 停用且已被用户引用时拒绝。返回受影响行数与 error。
func (s *RoleService) UpdateRoleBaseInfo(ctx context.Context,
	b *bo.SysRoleBo) (int64, error) {

	if b == nil {
		return 0, errors.New("service: 角色入参为空")
	}
	if b.RoleID <= 0 {
		return 0, errors.New("service: 角色主键不能为空")
	}
	if err := s.CheckRoleAllowed(ctx, b); err != nil {
		return 0, err
	}

	repo := repository.NewRoleRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"修改失败"。
	if _, err := repo.SelectByID(ctx, b.RoleID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return 0, ErrRoleNotFound
		}
		return 0, err
	}
	// 名称/权限重复校验，排除自身——对齐 Java edit 在 controller 里做的两道唯一性校验。
	if unique, err := s.CheckRoleNameUnique(ctx, b.RoleName, b.RoleID); err != nil {
		return 0, err
	} else if !unique {
		return 0, ErrRoleNameExists
	}
	if unique, err := s.CheckRoleKeyUnique(ctx, b.RoleKey, b.RoleID); err != nil {
		return 0, err
	} else if !unique {
		return 0, ErrRoleKeyExists
	}
	// 停用前确认没有用户引用：否则会让已分配该角色的用户挂在停用角色上。
	if b.Status == constant.StatusDisable {
		count, err := repo.CountUserRoleByRoleID(ctx, b.RoleID)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return 0, errs.New(0, "角色已分配，不能禁用!", "")
		}
	}

	rows, err := repo.UpdateByID(ctx, b.RoleID, buildRoleBaseUpdateColumns(b))
	if err != nil {
		return 0, err
	}
	// 仅在确实改了行时踢在线用户：对齐 Java 在 updateRoleBaseInfo > 0 分支里
	// 才 publish OnlineUserCleanEvent。no-op 重复保存不该把人踢下线。
	if rows > 0 {
		s.cleanOnlineUserByRole(ctx, b.RoleID)
	}
	return rows, nil
}

// UpdateRolePermission 修改角色权限（菜单 + 数据权限，对应 Java updateRolePermission）。
// 只更新 data_scope/树联动三列，先清旧菜单与部门关联再按当前勾选重建。
// 返回受影响行数与 error（含 @CacheEvict(SYS_ROLE_CUSTOM key=roleId) 的等价清理）。
func (s *RoleService) UpdateRolePermission(ctx context.Context,
	b *bo.SysRoleBo) (int64, error) {

	if b == nil {
		return 0, errors.New("service: 角色入参为空")
	}
	if b.RoleID <= 0 {
		return 0, errors.New("service: 角色主键不能为空")
	}
	if err := s.CheckRoleAllowed(ctx, b); err != nil {
		return 0, err
	}

	repo := repository.NewRoleRepository(database.DB())
	if _, err := repo.SelectByID(ctx, b.RoleID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return 0, ErrRoleNotFound
		}
		return 0, err
	}

	columns := map[string]any{
		"data_scope":          b.DataScope,
		"menu_check_strictly": b.MenuCheckStrictly,
		"dept_check_strictly": b.DeptCheckStrictly,
	}
	if _, err := repo.UpdateByID(ctx, b.RoleID, columns); err != nil {
		return 0, err
	}
	// 先清理旧菜单/部门关联再重建：清旧失败而重建成功会留下"旧授权未清掉"的脏状态，
	// 但此处无事务包裹，按 Java 的执行序照搬——重建失败则旧授权已成空，重试即可收敛。
	if _, err := repo.DeleteRoleMenuByRoleIDs(ctx, []int64{b.RoleID}); err != nil {
		return 0, err
	}
	if err := s.insertRoleMenu(ctx, repo, b.RoleID, b.MenuIDs); err != nil {
		return 0, err
	}
	if _, err := repo.DeleteRoleDeptByRoleIDs(ctx, []int64{b.RoleID}); err != nil {
		return 0, err
	}
	if err := s.insertRoleDept(ctx, repo, b.RoleID, b.DeptIDs); err != nil {
		return 0, err
	}
	// 数据权限缓存按角色失效：对应 Java @CacheEvict(SYS_ROLE_CUSTOM key=roleId)。
	_ = cache.Evict(ctx, constant.CacheSysRoleCustom, strconv.FormatInt(b.RoleID, 10))
	// 权限变更后让受影响在线用户下次请求即失效。
	s.cleanOnlineUserByRole(ctx, b.RoleID)
	// 返回值对齐 Java updateRolePermission 末尾 return insertRoleDept(bo)：
	// 取部门授权条数，空集合视作 1（成功），使 controller 的 toAjax(>0) 恒为成功口径。
	rows := int64(len(b.DeptIDs))
	if rows == 0 {
		rows = 1
	}
	return rows, nil
}

// UpdateRoleStatus 修改角色状态（对应 Java updateRoleStatus）。
// 停用且已被用户引用时拒绝。返回受影响行数与 error。
func (s *RoleService) UpdateRoleStatus(ctx context.Context,
	roleID int64, status string) (int64, error) {

	if roleID <= 0 {
		return 0, errors.New("service: 角色主键不能为空")
	}
	repo := repository.NewRoleRepository(database.DB())
	if status == constant.StatusDisable {
		count, err := repo.CountUserRoleByRoleID(ctx, roleID)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return 0, errs.New(0, "角色已分配，不能禁用!", "")
		}
	}

	rows, err := repo.UpdateByID(ctx, roleID, map[string]any{"status": status})
	if err != nil {
		return 0, err
	}
	// 仅在确实改了行时踢在线用户：对齐 Java 在 updateRoleStatus > 0 分支里
	// 才 publish OnlineUserCleanEvent。重复提交同一状态不该把人踢下线。
	if rows > 0 {
		s.cleanOnlineUserByRole(ctx, roleID)
	}
	return rows, nil
}

// DeleteRoleByIDs 批量删除角色（对应 Java deleteRoleByIds）。
// 先校验数据权限，再逐个校验「不可操作超管」「已分配用户不可删」，再清关联并删除。
// 任一校验失败整批拒绝，不做部分删除——此处无事务包裹，边删边校验会留下删一半的状态。
func (s *RoleService) DeleteRoleByIDs(ctx context.Context, userID int64,
	roleIDs []int64) error {

	if len(roleIDs) == 0 {
		return errors.New("service: 角色主键不能为空")
	}
	if err := s.checkRoleDataScopeAll(ctx, userID, roleIDs); err != nil {
		return err
	}

	repo := repository.NewRoleRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	for _, role := range rows {
		if err := s.CheckRoleAllowed(ctx, roleToBo(role)); err != nil {
			return err
		}
		count, err := repo.CountUserRoleByRoleID(ctx, role.RoleID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errs.New(0, fmt.Sprintf("%s已分配，不能删除", role.RoleName), "")
		}
	}

	// 先清关联再删主行：反序一旦中途失败会留下"角色已删、关联残留"的脏状态，
	// 但残留几行 sys_role_menu/role_dept 重新走一次级联删即可收敛，比反方向更易修。
	if _, err := repo.DeleteRoleMenuByRoleIDs(ctx, roleIDs); err != nil {
		return err
	}
	if _, err := repo.DeleteRoleDeptByRoleIDs(ctx, roleIDs); err != nil {
		return err
	}
	if _, err := repo.DeleteByIDs(ctx, roleIDs); err != nil {
		return err
	}
	// 批量删除整组清空数据权限缓存：对应 Java @CacheEvict(SYS_ROLE_CUSTOM allEntries=true)。
	_ = cache.EvictGroup(ctx, constant.CacheSysRoleCustom)
	return nil
}

// DeleteAuthUser 取消单个用户的角色授权（对应 Java deleteAuthUser）。
// 不允许取消当前登录用户自己的角色——否则会把自己踢出权限链导致无法操作。
// 取消成功后踢该用户下线，让其下次登录重取角色。返回受影响行数与 error。
func (s *RoleService) DeleteAuthUser(ctx context.Context, currentUserID,
	roleID, userID int64) (int64, error) {

	if userID == currentUserID {
		return 0, errs.New(0, "不允许修改当前用户角色!", "")
	}
	rows, err := repository.NewRoleRepository(database.DB()).
		DeleteUserRole(ctx, roleID, userID)
	if err != nil {
		return 0, err
	}
	if rows > 0 {
		loginhelper.LogoutUser(userID)
	}
	return rows, nil
}

// DeleteAuthUsers 批量取消角色下的用户授权（对应 Java deleteAuthUsers）。
// 当前登录用户不得在取消集合内。返回受影响行数与 error。
func (s *RoleService) DeleteAuthUsers(ctx context.Context, currentUserID,
	roleID int64, userIDs []int64) (int64, error) {

	if containsInt64(userIDs, currentUserID) {
		return 0, errs.New(0, "不允许修改当前用户角色!", "")
	}
	rows, err := repository.NewRoleRepository(database.DB()).
		DeleteUserRolesByRoleID(ctx, roleID, userIDs)
	if err != nil {
		return 0, err
	}
	if rows > 0 {
		loginhelper.LogoutUsers(userIDs)
	}
	return rows, nil
}

// InsertAuthUsers 批量给角色追加用户授权（对应 Java insertAuthUsers）。
// 当前登录用户不得在授权集合内——避免把自己加进角色后权限溢出。返回受影响行数与 error。
func (s *RoleService) InsertAuthUsers(ctx context.Context, roleID int64,
	userIDs []int64) (int64, error) {

	// rows 对齐 Java insertAuthUsers 的计数口径：空集合视作 1（成功），非空取入参数。
	rows := int64(len(userIDs))
	if rows == 0 {
		rows = 1
	}
	currentUserID := loginhelperFromCtx(ctx)
	if currentUserID != 0 && containsInt64(userIDs, currentUserID) {
		return 0, errs.New(0, "不允许修改当前用户角色!", "")
	}
	repo := repository.NewRoleRepository(database.DB())
	if err := repo.InsertUserRoles(ctx, roleID, userIDs); err != nil {
		return 0, err
	}
	if len(userIDs) > 0 {
		loginhelper.LogoutUsers(userIDs)
	}
	return rows, nil
}

// insertRoleMenu 批量写角色-菜单关联（对应 Java insertRoleMenu）。
// 空 list 直接返回：不勾任何菜单是合法用法。
func (s *RoleService) insertRoleMenu(ctx context.Context,
	repo *repository.RoleRepository, roleID int64, menuIDs []int64) error {

	if len(menuIDs) == 0 {
		return nil
	}
	list := make([]model.SysRoleMenu, 0, len(menuIDs))
	for _, mid := range menuIDs {
		list = append(list, model.SysRoleMenu{RoleID: roleID, MenuID: mid})
	}
	return repo.InsertRoleMenuBatch(ctx, list)
}

// insertRoleDept 批量写角色-部门（数据权限）关联（对应 Java insertRoleDept）。
// 空 list 直接返回：dataScope 非「自定义」的角色本就没有部门勾选。
func (s *RoleService) insertRoleDept(ctx context.Context,
	repo *repository.RoleRepository, roleID int64, deptIDs []int64) error {

	if len(deptIDs) == 0 {
		return nil
	}
	list := make([]model.SysRoleDept, 0, len(deptIDs))
	for _, did := range deptIDs {
		list = append(list, model.SysRoleDept{RoleID: roleID, DeptID: did})
	}
	return repo.InsertRoleDeptBatch(ctx, list)
}

// checkRoleDataScopeAll 批量数据权限校验（对应 Java checkRoleDataScope(Collection)）。
// 超管短路；非超管按主键统计可见数，与传入数量不等即越权。
func (s *RoleService) checkRoleDataScopeAll(ctx context.Context, userID int64,
	roleIDs []int64) error {

	if len(roleIDs) == 0 || loginhelper.IsSuperAdmin(userID) {
		return nil
	}
	count, err := repository.NewRoleRepository(database.DB()).
		CountByRoleIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	if count != int64(len(roleIDs)) {
		return errs.New(0, "没有权限访问部分角色数据！", "")
	}
	return nil
}

// cleanOnlineUserByRole 踢掉拥有该角色的在线用户（对应 Java cleanOnlineUserByRole，
// 由 OnlineUserCleanEvent.byRole 触发）。先取角色下的用户集合再逐个注销会话。
func (s *RoleService) cleanOnlineUserByRole(ctx context.Context, roleID int64) {
	userIDs, err := repository.NewRoleRepository(database.DB()).
		SelectUserIDsByRoleID(ctx, roleID)
	if err != nil {
		return
	}
	loginhelper.LogoutUsers(userIDs)
}

// buildRoleBaseUpdateColumns 组装角色基础信息更新列。data_scope/树联动不在其内——
// 那是权限域字段，由 UpdateRolePermission 承接，基础信息改动不该影响权限分配。
func buildRoleBaseUpdateColumns(b *bo.SysRoleBo) map[string]any {
	columns := map[string]any{
		"role_name": b.RoleName,
		"role_key":  b.RoleKey,
		"role_sort": b.RoleSort,
		// 一律写入，让前端能把备注清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"remark": b.Remark,
	}
	// 状态缺省即视为不改：漏传字段不该把线上的 '0' 刷成空串，
	// 那会让角色既不算启用也不算停用。等效于 Java updateById 对 null 字段的跳过。
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// roleToBo 把实体翻成 BO 供 CheckRoleAllowed 复用（对应 Java BeanUtil.toBean(role, SysRoleBo.class)）。
// 只填判定需要的 role_id/role_key/name，其余留零。
func roleToBo(r *model.SysRole) *bo.SysRoleBo {
	if r == nil {
		return nil
	}
	return &bo.SysRoleBo{
		RoleID:   r.RoleID,
		RoleName: r.RoleName,
		RoleKey:  r.RoleKey,
	}
}

// equalsAny 判断 s 是否等于 keys 中任一值，对齐 Java StringUtils.equalsAny。
func equalsAny(s string, keys []string) bool {
	for _, k := range keys {
		if s == k {
			return true
		}
	}
	return false
}

// containsInt64 判断 ids 是否含 v。
func containsInt64(ids []int64, v int64) bool {
	for _, id := range ids {
		if id == v {
			return true
		}
	}
	return false
}

// loginhelperFromCtx 从 ctx 取当前登录用户 ID。
//
// AuditContext 中间件把登录用户写进了请求 ctx（供 pkg/repository 审计回调用），
// 这里复用同一通道，免去再往 service 传 *gin.Context。未登录返回 0。
func loginhelperFromCtx(ctx context.Context) int64 {
	au, ok := pkgrepo.AuditUserFrom(ctx)
	if !ok {
		return 0
	}
	return au.UserID
}
