package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("repository: 用户不存在")

// UserRepository sys_user 数据访问。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造用户 repository。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// SelectByUserID 按用户ID查用户，不存在时返回 ErrUserNotFound。
func (r *UserRepository) SelectByUserID(ctx context.Context, userID int64) (*model.SysUser, error) {
	if userID == 0 {
		return nil, ErrUserNotFound
	}

	var user model.SysUser
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: 查询用户 %d 失败: %w", userID, err)
	}
	return &user, nil
}

// SelectByUserName 按用户账号查用户，不存在时返回 ErrUserNotFound。
func (r *UserRepository) SelectByUserName(ctx context.Context, userName string) (*model.SysUser, error) {
	if userName == "" {
		return nil, ErrUserNotFound
	}

	var user model.SysUser
	err := r.db.WithContext(ctx).
		Where("user_name = ?", userName).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: 查询用户 %q 失败: %w", userName, err)
	}
	return &user, nil
}

// SelectUserNamesByIDs 按用户ID批量取账号，返回 id → user_name 映射
// （对应 Java selectUserNameById 的批量形态，供 USER_ID_TO_NAME 翻译用）。
//
// 一次 IN 查询而非逐个查：列表页每行都要翻译创建人，单查会打出 N+1。
// 缺失的 ID 不出现在结果里，由调用方按空串兜底。
func (r *UserRepository) SelectUserNamesByIDs(ctx context.Context,
	userIDs []int64) (map[int64]string, error) {

	if len(userIDs) == 0 {
		return map[int64]string{}, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var rows []struct {
		UserID   int64
		UserName string
	}
	err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Select("user_id", "user_name").
		Where("user_id IN ?", userIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 批量查询用户账号 %v 失败: %w", userIDs, err)
	}

	names := make(map[int64]string, len(rows))
	for _, row := range rows {
		names[row.UserID] = row.UserName
	}
	return names, nil
}

// ExistsByDeptID 判断部门下是否已分配用户（对应 Java checkDeptExistUser）。
func (r *UserRepository) ExistsByDeptID(ctx context.Context, deptID int64) (bool, error) {
	if deptID <= 0 {
		return false, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("dept_id = ?", deptID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询部门 %d 下的用户失败: %w", deptID, err)
	}
	return count > 0, nil
}

// UpdateLoginInfo 更新最后登录 IP 与时间。
func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userID int64, ip string) error {
	if userID == 0 {
		return errors.New("repository: userID 不能为 0")
	}

	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"login_ip":    ip,
			"login_date":  now,
			"update_by":   userID,
			"update_time": now,
		}).Error
	if err != nil {
		return fmt.Errorf("repository: 更新用户 %d 登录信息失败: %w", userID, err)
	}
	return nil
}

// ExistsByPhone 判断手机号是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkPhoneUnique 的 neIfPresent，供个人中心改资料排除自身）。
func (r *UserRepository) ExistsByPhone(ctx context.Context, phone string,
	excludeID int64) (bool, error) {

	if phone == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("phone_number = ?", phone)
	if excludeID > 0 {
		db = db.Where("user_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验手机号 %q 失败: %w", phone, err)
	}
	return count > 0, nil
}

// ExistsByEmail 判断邮箱是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkEmailUnique 的 neIfPresent，供个人中心改资料排除自身）。
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string,
	excludeID int64) (bool, error) {

	if email == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("email = ?", email)
	if excludeID > 0 {
		db = db.Where("user_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验邮箱 %q 失败: %w", email, err)
	}
	return count > 0, nil
}

// UpdateProfile 按主键更新个人资料（对应 Java updateUserProfile）。
//
// 走 map 且仅写非空字段——对齐 Java setIfPresent：个人资料表单字段全可选，
// 漏传的字段不该把线上的昵称/邮箱刷成空串。update_by/update_time 由回调补齐。
func (r *UserRepository) UpdateProfile(ctx context.Context, userID int64,
	columns map[string]any) (int64, error) {

	if userID == 0 {
		return 0, errors.New("repository: userID 不能为 0")
	}
	if len(columns) == 0 {
		return 0, errors.New("repository: 个人资料更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新用户 %d 个人资料失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdatePassword 按主键重置密码（对应 Java resetUserPwd）。
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64,
	password string) (int64, error) {

	if userID == 0 {
		return 0, errors.New("repository: userID 不能为 0")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Update("password", password)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 重置用户 %d 密码失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// SelectAllocatedList 分页查询已分配某角色的用户（对应 Java selectAllocatedList）。
//
// 联 sys_user_role 取授权关系，按 sur.role_id 过滤；一个用户对一个角色仅一行 sur，
// 故无需 DISTINCT——避免 GORM 的 Count 不识别 DISTINCT、把去重后的行数错算成去重前
// （联表会因用户的多角色把同一用户拉成多行）。userName/status/phoneNumber 走共用过滤。
// Java 此处带 @DataPermission 注入部门/创建人隔离条件，Go 侧数据权限尚未落地，
// 此处只按业务条件查——等数据权限落地后挂上过滤，调用点不必改。
func (r *UserRepository) SelectAllocatedList(ctx context.Context, q bo.SysUserQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysUser], error) {

	db := r.applyUserAuthQuery(r.db.WithContext(ctx).Model(&model.SysUser{}), q)
	if q.RoleID > 0 {
		db = db.
			Joins("JOIN sys_user_role sur ON sur.user_id = sys_user.user_id").
			Where("sur.role_id = ?", q.RoleID)
	}
	if !page.HasOrder() {
		// 对齐 Java orderByAsc("u", SysUser::getUserId)。
		db = db.Order("sys_user.user_id")
	}

	var rows []*model.SysUser
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectUnallocatedList 分页查询未分配某角色的用户（对应 Java selectUnallocatedList）。
// excludeUserIDs 是已分配该角色的用户集合（由 service 先取一次），NOT IN 排除之。
//
// 不联 sys_user_role：联表会让多角色用户拉成多行，而 GORM 的 Count 不识别 DISTINCT
// 会把总数错算成去重前。直接 NOT IN 主键集合即可——每用户一行，Count 与 Find 一致。
// 与 Java 的 LEFT JOIN sur 行为一致：没有任何角色授权的用户也会出现在未分配列表里。
func (r *UserRepository) SelectUnallocatedList(ctx context.Context, q bo.SysUserQueryBo,
	excludeUserIDs []int64, page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysUser], error) {

	db := r.applyUserAuthQuery(r.db.WithContext(ctx).Model(&model.SysUser{}), q)
	if len(excludeUserIDs) > 0 {
		db = db.Where("sys_user.user_id NOT IN ?", excludeUserIDs)
	}
	if !page.HasOrder() {
		db = db.Order("sys_user.user_id")
	}

	var rows []*model.SysUser
	return pkgrepo.SelectPage(db, page, &rows)
}

// applyUserAuthQuery 应用 allocated/unallocated 共用的用户过滤条件
// （对应 Java buildUserRoleJoinWrapper 的 likeIfText/eqIfText 部分）。
// 基表是 sys_user（无别名），条件均以 sys_user 前缀限定，避免与联表同名列冲突。
// 不在此处加 JOIN：两条路径的联表结构不同（allocated 联 sur、unallocated 不联），
// 各自在调用方补，共用此函数只承载过滤条件。
func (r *UserRepository) applyUserAuthQuery(db *gorm.DB, q bo.SysUserQueryBo) *gorm.DB {
	if q.UserName != "" {
		db = db.Where("sys_user.user_name LIKE ?", "%"+escapeLike(q.UserName)+"%")
	}
	if q.Status != "" {
		db = db.Where("sys_user.status = ?", q.Status)
	}
	if q.PhoneNumber != "" {
		db = db.Where("sys_user.phone_number LIKE ?", "%"+escapeLike(q.PhoneNumber)+"%")
	}
	return db
}

// splitInt64s 拆分逗号分隔的 ID 串为 int64 切片，丢弃空白与非法段
// （对应 Java StringUtils.splitTo(s, Convert::toLong)：解析失败整段作废，
// 这里与 controller 的 parseIDs 同口径，保持 list 过滤的主键集合口径一致）。
func splitInt64s(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil
		}
		out = append(out, id)
	}
	return out
}

// applyUserListQuery 应用用户管理列表/导出共用的过滤条件
// （对应 Java SysUserServiceImpl.buildQueryWrapper）。
// userName/nickName/phoneNumber 走 LIKE，status 走 =，userId 走 =，
// userIds/excludeUserIds 走 IN/NOT IN（解析逗号串），createTime 走闭区间，
// deptIds 走 IN（由 service 按 DeptID 解析成「自身+子部门」后填入）。
//
// 与 applyUserAuthQuery 分开：列表过滤更全且基表不联 sur，授权页过滤是其子集。
// 分页与导出两条路径必须共用本函数，否则过滤逻辑改一处漏一处。
func (r *UserRepository) applyUserListQuery(db *gorm.DB, q bo.SysUserQueryBo) *gorm.DB {
	if q.UserName != "" { // likeIfText：空串不筛
		db = db.Where("sys_user.user_name LIKE ?", "%"+escapeLike(q.UserName)+"%")
	}
	if q.NickName != "" {
		db = db.Where("sys_user.nick_name LIKE ?", "%"+escapeLike(q.NickName)+"%")
	}
	if q.Status != "" { // eqIfText：空串不筛
		db = db.Where("sys_user.status = ?", q.Status)
	}
	if q.PhoneNumber != "" {
		db = db.Where("sys_user.phone_number LIKE ?", "%"+escapeLike(q.PhoneNumber)+"%")
	}
	if q.UserID > 0 { // eqIfPresent：0 不筛
		db = db.Where("sys_user.user_id = ?", q.UserID)
	}
	if ids := splitInt64s(q.UserIDs); len(ids) > 0 {
		db = db.Where("sys_user.user_id IN ?", ids)
	}
	if ids := splitInt64s(q.ExcludeUserIDs); len(ids) > 0 {
		db = db.Where("sys_user.user_id NOT IN ?", ids)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("sys_user.create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	// deptId 非 0 时 service 已解析成 DeptIDs（自身+全部子部门），按集合 IN 过滤。
	if len(q.DeptIDs) > 0 {
		db = db.Where("sys_user.dept_id IN ?", q.DeptIDs)
	}
	return db
}

// SelectPageList 分页查询用户管理列表（对应 Java selectPageUserList）。
//
// Java 此处带 @DataPermission 注入部门/角色数据隔离条件，Go 侧数据权限尚未落地，
// 此处只按业务条件查——等数据权限落地后挂上过滤，调用点不必改。
func (r *UserRepository) SelectPageList(ctx context.Context, q bo.SysUserQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysUser], error) {

	db := r.applyUserListQuery(r.db.WithContext(ctx).Model(&model.SysUser{}), q)
	if !page.HasOrder() {
		// 对齐 Java orderByAsc(SysUser::getUserId)。
		db = db.Order("sys_user.user_id")
	}
	var rows []*model.SysUser
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectExportList 按条件查导出列表，limit <= 0 不限制；导出方应传 excel.MaxRows+1
// 以提前判定超限，见 pkg/excel 的说明。
func (r *UserRepository) SelectExportList(ctx context.Context, q bo.SysUserQueryBo,
	limit int) ([]*model.SysUser, error) {

	db := r.applyUserListQuery(r.db.WithContext(ctx).Model(&model.SysUser{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定排序保证输出顺序稳定。
	db = db.Order("sys_user.user_id")
	if limit > 0 {
		db = db.Limit(limit)
	}
	var rows []*model.SysUser
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询用户导出列表失败: %w", err)
	}
	return rows, nil
}

// SelectByIDs 按用户ID串/部门取基础信息（对应 Java selectUserByIds）。
// 仅取 user_id/user_name/nick_name 三列，固定 status='0'（启用），
// userIds 非空时按 IN 过滤、deptID > 0 时按部门精确过滤。两者皆空则返回全部启用用户。
func (r *UserRepository) SelectByIDs(ctx context.Context, userIds []int64,
	deptID int64) ([]*model.SysUser, error) {

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	db := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Select("user_id", "user_name", "nick_name").
		Where("status = ?", "0")
	if len(userIds) > 0 {
		db = db.Where("user_id IN ?", userIds)
	}
	if deptID > 0 {
		db = db.Where("dept_id = ?", deptID)
	}
	db = db.Order("user_id")

	var rows []*model.SysUser
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %v 失败: %w", userIds, err)
	}
	return rows, nil
}

// ExistsByUserName 判断用户账号是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkUserNameUnique 的 neIfPresent，供修改场景排除自身）。
func (r *UserRepository) ExistsByUserName(ctx context.Context, userName string,
	excludeID int64) (bool, error) {

	if userName == "" {
		return false, nil
	}
	db := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("user_name = ?", userName)
	if excludeID > 0 {
		db = db.Where("user_id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验账号 %q 失败: %w", userName, err)
	}
	return count > 0, nil
}

// CountByUserID 按主键统计存在的用户数（对应 Java countUserById）。
// 用于数据权限校验：可见数与传入数不等即越权。
//
// Java 此方法带 @DataPermission，非超管账号下注入隔离条件，使"我能看到的用户"与
// 传入主键不等时抛"没有权限访问用户数据"。Go 侧数据权限尚未落地，此处只按主键计数——
// 超管在 service 层短路，非超管传入的 userId 都是前端可见的，count 不等只能说明主键不存在。
// 等数据权限落地后给这次 count 挂上过滤即可，调用点不必改。
func (r *UserRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("repository: 统计用户 %d 失败: %w", userID, err)
	}
	return count, nil
}

// SelectListByDept 按部门取全部用户（对应 Java selectUserListByDept）。
func (r *UserRepository) SelectListByDept(ctx context.Context,
	deptID int64) ([]*model.SysUser, error) {

	if deptID <= 0 {
		return nil, nil
	}
	var rows []*model.SysUser
	if err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("dept_id = ?", deptID).
		Order("user_id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询部门 %d 的用户失败: %w", deptID, err)
	}
	return rows, nil
}

// Insert 插入一条用户。
// user_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *UserRepository) Insert(ctx context.Context, user *model.SysUser) error {
	if user == nil {
		return fmt.Errorf("repository: 用户为空")
	}
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("repository: 插入用户失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *UserRepository) UpdateByID(ctx context.Context, userID int64,
	columns map[string]any) (int64, error) {

	if userID <= 0 {
		return 0, fmt.Errorf("repository: 用户主键无效: %d", userID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 用户更新列为空")
	}
	res := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新用户 id=%d 失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateStatus 按主键更新账号状态（对应 Java updateUserStatus）。
func (r *UserRepository) UpdateStatus(ctx context.Context, userID int64,
	status string) (int64, error) {

	return r.UpdateByID(ctx, userID, map[string]any{"status": status})
}

// DeleteByIDs 按主键批量逻辑删除，返回受影响行数。
// SysUser 嵌了 LogicDelete，Delete 会被改写成 UPDATE ... SET del_flag = '1'。
func (r *UserRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 用户主键为空")
	}
	res := r.db.WithContext(ctx).
		Where("user_id IN ?", ids).
		Delete(&model.SysUser{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除用户 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// InsertUserRoles 批量写入用户-角色关联（对应 Java insertUserRole 的批量形态）。
// 空 list 直接返回：不勾任何角色走不到这里（service 已校验非空），保留幂等。
func (r *UserRepository) InsertUserRoles(ctx context.Context, userID int64,
	roleIDs []int64) error {

	if userID <= 0 || len(roleIDs) == 0 {
		return nil
	}
	list := make([]model.SysUserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		list = append(list, model.SysUserRole{UserID: userID, RoleID: rid})
	}
	if err := r.db.WithContext(ctx).CreateInBatches(list, len(list)).Error; err != nil {
		return fmt.Errorf("repository: 批量插入用户 %d 的角色关联失败: %w", userID, err)
	}
	return nil
}

// DeleteUserRolesByUserID 清理用户的全部角色关联（对应 Java updateUser 的清旧分支）。
// sys_user_role 无 del_flag，物理删除。
func (r *UserRepository) DeleteUserRolesByUserID(ctx context.Context,
	userID int64) (int64, error) {

	if userID <= 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.SysUserRole{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理用户 %d 的角色关联失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// InsertUserPosts 批量写入用户-岗位关联（对应 Java insertUserPost 的批量形态）。
func (r *UserRepository) InsertUserPosts(ctx context.Context, userID int64,
	postIDs []int64) error {

	if userID <= 0 || len(postIDs) == 0 {
		return nil
	}
	list := make([]model.SysUserPost, 0, len(postIDs))
	for _, pid := range postIDs {
		list = append(list, model.SysUserPost{UserID: userID, PostID: pid})
	}
	if err := r.db.WithContext(ctx).CreateInBatches(list, len(list)).Error; err != nil {
		return fmt.Errorf("repository: 批量插入用户 %d 的岗位关联失败: %w", userID, err)
	}
	return nil
}

// DeleteUserPostsByUserID 清理用户的全部岗位关联（对应 Java updateUser 的清旧分支）。
// sys_user_post 无 del_flag，物理删除。
func (r *UserRepository) DeleteUserPostsByUserID(ctx context.Context,
	userID int64) (int64, error) {

	if userID <= 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.SysUserPost{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理用户 %d 的岗位关联失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteUserRolesByUserIDs 批量清理给定用户的角色关联（对应 Java deleteUserByIds 的清旧分支）。
func (r *UserRepository) DeleteUserRolesByUserIDs(ctx context.Context,
	userIDs []int64) (int64, error) {

	if len(userIDs) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Delete(&model.SysUserRole{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理用户 %v 的角色关联失败: %w", userIDs, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteUserPostsByUserIDs 批量清理给定用户的岗位关联（对应 Java deleteUserByIds 的清旧分支）。
func (r *UserRepository) DeleteUserPostsByUserIDs(ctx context.Context,
	userIDs []int64) (int64, error) {

	if len(userIDs) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Delete(&model.SysUserPost{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 清理用户 %v 的岗位关联失败: %w", userIDs, res.Error)
	}
	return res.RowsAffected, nil
}
