package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/bcrypt"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/redis"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/snowflake"
	"ruoyi-go-vue-plus/pkg/tree"
)

// ErrUserPhoneExists 手机号已被占用。
var ErrUserPhoneExists = errors.New("service: 手机号已存在")

// ErrUserEmailExists 邮箱已被占用。
var ErrUserEmailExists = errors.New("service: 邮箱已存在")

// ErrUserNameExists 登录账号已被占用。
var ErrUserNameExists = errors.New("service: 登录账号已存在")

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("service: 用户不存在")

// ErrUserUpdateFailed 修改用户未生效（对齐 Java flag < 1 的失败口径）。
var ErrUserUpdateFailed = errors.New("service: 修改用户信息失败")

// ErrUserDeleteFailed 删除用户未生效（对齐 Java flag < 1 的失败口径）。
var ErrUserDeleteFailed = errors.New("service: 删除用户失败")

// ErrUserProfileUpdate 个人资料更新未生效（对齐 Java rows <= 0 的失败口径）。
var ErrUserProfileUpdate = errors.New("service: 修改个人信息异常，请联系管理员")

// ErrUserPasswordSame 新旧密码相同。
var ErrUserPasswordSame = errors.New("service: 新密码不能与旧密码相同")

// ErrUserPasswordWrong 旧密码错误。
var ErrUserPasswordWrong = errors.New("service: 旧密码错误")

// UserService 用户业务逻辑。
type UserService struct{}

// UserSvcApp 包级实例。
var UserSvcApp = new(UserService)

// LoadUserByUsername 按用户名加载可登录用户，并校验是否存在或被停用
// （对应 Java PasswordAuthStrategy#loadUserByUsername）。
func (*UserService) LoadUserByUsername(ctx context.Context, username string) (*vo.SysUserVo, error) {
	entity, err := repository.NewUserRepository(database.DB()).SelectByUserName(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			log.Printf("[auth] 登录用户: %s 不存在", username)
			return nil, errs.New(0, i18n.Msg(ctx, "user.not.exists", username), "")
		}
		return nil, err
	}
	user := vo.Conv.ConvertToSysUserVo(entity)
	if user.Status == enum.UserStatusDisable.Code {
		log.Printf("[auth] 登录用户: %s 已被停用", username)
		return nil, errs.New(0, i18n.Msg(ctx, "user.blocked", username), "")
	}
	return user, nil
}

// SelectUserByID 按用户ID查用户并回填角色（对应 Java ISysUserService#selectUserById）。
// Java 原用 DataPermissionHelper.ignore 跳过数据权限隔离；Go 侧数据权限尚未落地，
// 此处直接查库。用户不存在返回 (nil, nil)，由 handler 转成"没有权限访问用户数据"。
func (*UserService) SelectUserByID(ctx context.Context, userID int64) (*vo.SysUserVo, error) {
	entity, err := repository.NewUserRepository(database.DB()).SelectByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}
	user := vo.Conv.ConvertToSysUserVo(entity)
	roles, err := RoleSvcApp.SelectRolesByUserId(ctx, user.UserID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return user, nil
}

// SelectUserProfile 取个人中心信息（对应 Java SysProfileController.profile）。
// 单独走 ProfileUserVo 而非 SysUserVo：个人中心要避开脱敏字段，并附带角色组/岗位组。
//
// 用户不存在返回 (nil, nil)：当前登录用户查不到属于异常登录态，由 handler 兜底提示。
func (*UserService) SelectUserProfile(ctx context.Context,
	userID int64) (*vo.ProfileVo, error) {

	entity, err := repository.NewUserRepository(database.DB()).SelectByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}

	user := vo.Conv.ConvertToProfileUserVo(entity)
	// 回填部门名：Java 由 @Translation(DEPT_ID_TO_NAME) 在 VO 层完成，Go 无翻译层，手动查。
	// 父级不存在（如根部门）时留空，不算错。
	if user.DeptID > 0 {
		if dept, err := repository.NewDeptRepository(database.DB()).
			SelectByID(ctx, user.DeptID); err == nil {
			user.DeptName = dept.DeptName
		} else if !errors.Is(err, repository.ErrDeptNotFound) {
			return nil, err
		}
	}

	roleGroup, err := UserSvcApp.SelectUserRoleGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	postGroup, err := UserSvcApp.SelectUserPostGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &vo.ProfileVo{User: user, RoleGroup: roleGroup, PostGroup: postGroup}, nil
}

// SelectUserRoleGroup 查用户所属角色组（对应 Java selectUserRoleGroup）。
// 把角色名按逗号拼接；无角色返回空串（对齐 Java StringUtils.EMPTY）。
func (*UserService) SelectUserRoleGroup(ctx context.Context, userID int64) (string, error) {
	roles, err := RoleSvcApp.SelectRolesByUserId(ctx, userID)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		if r != nil && r.RoleName != "" {
			names = append(names, r.RoleName)
		}
	}
	return strings.Join(names, ","), nil
}

// SelectUserPostGroup 查用户所属岗位组（对应 Java selectUserPostGroup）。
// 把岗位名按逗号拼接；无岗位返回空串。
func (*UserService) SelectUserPostGroup(ctx context.Context, userID int64) (string, error) {
	posts, err := PostSvcApp.SelectPostsByUserId(ctx, userID)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(posts))
	for _, p := range posts {
		if p != nil && p.PostName != "" {
			names = append(names, p.PostName)
		}
	}
	return strings.Join(names, ","), nil
}

// CheckPhoneUnique 校验手机号是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供个人中心改资料排除自身。
func (*UserService) CheckPhoneUnique(ctx context.Context, phone string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewUserRepository(database.DB()).
		ExistsByPhone(ctx, phone, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckEmailUnique 校验邮箱是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供个人中心改资料排除自身。
func (*UserService) CheckEmailUnique(ctx context.Context, email string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewUserRepository(database.DB()).
		ExistsByEmail(ctx, email, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// UpdateUserProfile 修改个人资料（对应 Java updateUserProfile）。
// 手机号/邮箱非空时先校验唯一；更新未生效（rows=0）返回 ErrUserProfileUpdate。
func (*UserService) UpdateUserProfile(ctx context.Context, userID int64,
	b *bo.SysUserProfileBo) error {

	if b == nil || userID == 0 {
		return errors.New("service: 个人资料入参为空")
	}

	// 仅在传入非空值时校验唯一：对齐 Java isNotEmpty 判空后再 check。
	if b.PhoneNumber != "" {
		unique, err := UserSvcApp.CheckPhoneUnique(ctx, b.PhoneNumber, userID)
		if err != nil {
			return err
		}
		if !unique {
			return ErrUserPhoneExists
		}
	}
	if b.Email != "" {
		unique, err := UserSvcApp.CheckEmailUnique(ctx, b.Email, userID)
		if err != nil {
			return err
		}
		if !unique {
			return ErrUserEmailExists
		}
	}

	columns := buildUserProfileColumns(b)
	rows, err := repository.NewUserRepository(database.DB()).
		UpdateProfile(ctx, userID, columns)
	if err != nil {
		return err
	}
	// 当前登录用户查不到属异常态；rows=0 对齐 Java 的 toAjax(0) 失败口径。
	if rows == 0 {
		return ErrUserProfileUpdate
	}
	return nil
}

// buildUserProfileColumns 组装个人资料更新列。
//
// 走 map 且仅写非空字段——对齐 Java setIfPresent：个人资料表单字段全可选，
// 漏传的字段不该把线上的昵称/邮箱刷成空串。avatar 为 0 视为未传。
func buildUserProfileColumns(b *bo.SysUserProfileBo) map[string]any {
	columns := make(map[string]any, 5)
	if b.NickName != "" {
		columns["nick_name"] = b.NickName
	}
	if b.Email != "" {
		columns["email"] = b.Email
	}
	if b.PhoneNumber != "" {
		columns["phone_number"] = b.PhoneNumber
	}
	if b.Gender != "" {
		columns["gender"] = b.Gender
	}
	if b.Avatar > 0 {
		columns["avatar"] = b.Avatar
	}
	return columns
}

// ResetUserPwd 重置密码（对应 Java resetUserPwd）。
// 明文由调用方传入，此处负责 BCrypt 哈希后落库。
func (*UserService) ResetUserPwd(ctx context.Context, userID int64,
	password string) error {

	hashed, err := bcrypt.Hashpw(password)
	if err != nil {
		return err
	}
	rows, err := repository.NewUserRepository(database.DB()).
		UpdatePassword(ctx, userID, hashed)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.New(0, "修改密码异常，请联系管理员", "")
	}
	return nil
}

// ChangeUserPassword 修改当前用户密码（对应 Java SysProfileController.updatePwd）。
// 旧密码不匹配返回 ErrUserPasswordWrong，新旧相同返回 ErrUserPasswordSame，
// 校验通过后由 ResetUserPwd 哈希落库。只取一次用户记录，旧/新两次校验共用同一份哈希。
func (*UserService) ChangeUserPassword(ctx context.Context, userID int64,
	oldPassword, newPassword string) error {

	entity, err := repository.NewUserRepository(database.DB()).SelectByUserID(ctx, userID)
	if err != nil {
		// 当前登录用户查不到属异常态，按旧密码错误兜底，避免泄露用户存在性。
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserPasswordWrong
		}
		return err
	}

	// 旧密码校验：不匹配即 ErrUserPasswordWrong，对齐 Java BCrypt.checkpw 失败分支。
	if err := bcrypt.Verify(oldPassword, entity.Password); err != nil {
		return ErrUserPasswordWrong
	}
	// 新密码不能与旧密码相同：新明文能匹配旧哈希即视为相同。
	if bcrypt.Verify(newPassword, entity.Password) == nil {
		return ErrUserPasswordSame
	}
	return UserSvcApp.ResetUserPwd(ctx, userID, newPassword)
}

// SelectAllocatedList 分页查询已分配某角色的用户（对应 Java selectAllocatedList）。
// roleId 由 q.RoleID 承载；空结果不报错。返回 VO，不回填部门名（DEPT_ID_TO_NAME
// 翻译层尚未落地，与 SelectUserByID 一致留空）。
func (*UserService) SelectAllocatedList(ctx context.Context, q bo.SysUserQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysUserVo], error) {

	res, err := repository.NewUserRepository(database.DB()).
		SelectAllocatedList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysUserVo](), err
	}
	return pkgrepo.Page(vo.Conv.ConvertToSysUserVoList(res.Rows), res.Total), nil
}

// SelectUnallocatedList 分页查询未分配某角色的用户（对应 Java selectUnallocatedList）。
// 先取该角色已分配的用户集合，再 NOT IN 排除——与 Java 的两步一致。
func (*UserService) SelectUnallocatedList(ctx context.Context, q bo.SysUserQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysUserVo], error) {

	excludeUserIDs, err := repository.NewRoleRepository(database.DB()).
		SelectUserIDsByRoleID(ctx, q.RoleID)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysUserVo](), err
	}
	res, err := repository.NewUserRepository(database.DB()).
		SelectUnallocatedList(ctx, q, excludeUserIDs, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysUserVo](), err
	}
	return pkgrepo.Page(vo.Conv.ConvertToSysUserVoList(res.Rows), res.Total), nil
}

// resolveDeptIDs 把 q.DeptID 解析成「自身+全部子部门」写回 q.DeptIDs
// 供 repository 的 IN 过滤用（对应 Java buildQueryWrapper 里 selectDeptAndChildById 的子查询）。
func (s *UserService) resolveDeptIDs(ctx context.Context, q *bo.SysUserQueryBo) error {
	if q.DeptID <= 0 {
		return nil
	}
	ids, err := repository.NewDeptRepository(database.DB()).SelectDeptAndChildIDs(ctx, q.DeptID)
	if err != nil {
		return err
	}
	q.DeptIDs = ids
	return nil
}

// QueryPageList 分页查询用户管理列表（对应 Java selectPageUserList）。
func (s *UserService) QueryPageList(ctx context.Context, q bo.SysUserQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysUserVo], error) {

	if err := s.resolveDeptIDs(ctx, &q); err != nil {
		return pkgrepo.EmptyPage[*vo.SysUserVo](), err
	}
	res, err := repository.NewUserRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysUserVo](), err
	}
	return pkgrepo.Page(vo.Conv.ConvertToSysUserVoList(res.Rows), res.Total), nil
}

// QueryExportList 按条件查导出列表（对应 Java selectUserExportList）。
// limit <= 0 不限制；导出方应传 excel.MaxRows+1 以提前判定超限。
// 批量回填 DeptName/LeaderName：先按结果里的 deptID 集合查部门（拿部门名+负责人 userID），
// 再按负责人 userID 集合一次查账号名（对齐 Java 联表取 leader 的 user_name），两查询与行数无关，避免 N+1。
func (s *UserService) QueryExportList(ctx context.Context, q bo.SysUserQueryBo,
	limit int) ([]*vo.SysUserExportVo, error) {

	if err := s.resolveDeptIDs(ctx, &q); err != nil {
		return nil, err
	}
	rows, err := repository.NewUserRepository(database.DB()).SelectExportList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := vo.Conv.ConvertToSysUserExportVoList(rows)
	fillExportDeptLeader(ctx, out)
	return out, nil
}

// fillExportDeptLeader 回填导出行的部门名与负责人名。
// Java 用 DeptExcelConverter 把 deptId 渲染成部门名、联表取 leader 的 user_name；
// Go 无翻译层，直接按集合查库回填。部门/负责人查不到时留空，不算错（fail-open）。
func fillExportDeptLeader(ctx context.Context, rows []*vo.SysUserExportVo) {
	deptIDs := make(map[int64]struct{})
	for _, r := range rows {
		if r != nil && r.DeptID > 0 {
			deptIDs[r.DeptID] = struct{}{}
		}
	}
	if len(deptIDs) == 0 {
		return
	}

	deptRepo := repository.NewDeptRepository(database.DB())
	type deptInfo struct {
		name   string
		leader int64
	}
	deptMap := make(map[int64]deptInfo, len(deptIDs))
	leaderSet := make(map[int64]struct{})
	for did := range deptIDs {
		d, err := deptRepo.SelectByID(ctx, did)
		if err != nil {
			if errors.Is(err, repository.ErrDeptNotFound) {
				continue
			}
			return // 查询出错时整组留空，不阻断导出
		}
		deptMap[did] = deptInfo{name: d.DeptName, leader: d.Leader}
		if d.Leader > 0 {
			leaderSet[d.Leader] = struct{}{}
		}
	}

	leaders := make([]int64, 0, len(leaderSet))
	for lid := range leaderSet {
		leaders = append(leaders, lid)
	}
	names, err := repository.NewUserRepository(database.DB()).SelectUserNamesByIDs(ctx, leaders)
	if err != nil {
		return
	}
	for _, r := range rows {
		if r == nil {
			continue
		}
		if di, ok := deptMap[r.DeptID]; ok {
			r.DeptName = di.name
			r.LeaderName = names[di.leader] // 缺失负责人→空串
		}
	}
}

// ImportUsers 导入用户（对应 Java SysUserImportListener）。
// 读 xlsx 后逐行处理：不存在则新增（密码取 sys.user.initPassword 配置，由 InsertUser BCrypt），
// 存在且 updateSupport 则更新（保留既有角色/岗位——空集不清旧），否则记"已存在"。
// 任一行异常只记失败不中断；failureNum>0 抛汇总错误，否则返回成功汇总文案。
func (s *UserService) ImportUsers(ctx context.Context, r io.Reader,
	updateSupport bool) (string, error) {

	rows, err := excel.Read[vo.SysUserImportVo](r)
	if err != nil {
		return "", err
	}
	initPwd, err := ConfigSvcApp.SelectConfigByKey(ctx, "sys.user.initPassword")
	if err != nil {
		return "", err
	}

	successNum, failureNum := 0, 0
	var successMsg, failureMsg strings.Builder
	for _, iv := range rows {
		userName := strings.TrimSpace(iv.UserName)
		// 基础格式校验对齐 Java @NotBlank @Size(2-30)：账号缺失或超长直接判失败行。
		if userName == "" || len(userName) < 2 || len(userName) > 30 {
			failureNum++
			failureMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 导入失败：用户账号不能为空或长度不在2到30之间",
				failureNum, userName))
			continue
		}

		existing, err := repository.NewUserRepository(database.DB()).SelectByUserName(ctx, userName)
		if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
			failureNum++
			failureMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 导入失败：%s",
				failureNum, userName, err.Error()))
			continue
		}

		if existing == nil {
			b := &bo.SysUserBo{
				UserID:      iv.UserID,
				DeptID:      iv.DeptID,
				UserName:    userName,
				NickName:    iv.NickName,
				Email:       iv.Email,
				PhoneNumber: iv.PhoneNumber,
				Gender:      iv.Gender,
				Status:      iv.Status,
				Password:    initPwd, // 明文，InsertUser 内部 BCrypt
			}
			if err := s.InsertUser(ctx, b); err != nil {
				failureNum++
				failureMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 导入失败：%s",
					failureNum, userName, err.Error()))
				continue
			}
			successNum++
			successMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 导入成功", successNum, userName))
		} else if updateSupport {
			b := &bo.SysUserBo{
				UserID:      existing.UserID,
				DeptID:      iv.DeptID,
				UserName:    userName,
				NickName:    iv.NickName,
				Email:       iv.Email,
				PhoneNumber: iv.PhoneNumber,
				Gender:      iv.Gender,
				Status:      iv.Status,
			}
			if err := s.UpdateUser(ctx, b); err != nil {
				failureNum++
				failureMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 导入失败：%s",
					failureNum, userName, err.Error()))
				continue
			}
			successNum++
			successMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 更新成功", successNum, userName))
		} else {
			failureNum++
			failureMsg.WriteString(fmt.Sprintf("\n%d、账号 %s 已存在", failureNum, userName))
		}
	}

	if failureNum > 0 {
		return "", errs.New(0, fmt.Sprintf("很抱歉，导入失败！共 %d 条数据格式不正确，错误如下：%s",
			failureNum, failureMsg.String()), "")
	}
	return fmt.Sprintf("恭喜您，数据已全部导入成功！共 %d 条，数据如下：%s",
		successNum, successMsg.String()), nil
}

// CheckUserNameUnique 校验登录账号是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景排除自身。
func (s *UserService) CheckUserNameUnique(ctx context.Context, userName string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewUserRepository(database.DB()).
		ExistsByUserName(ctx, userName, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckUserAllowed 校验用户是否允许操作（对应 Java checkUserAllowed）。
// 超管用户不可被增删改。失败时返回带文案的 ServiceError。
func (s *UserService) CheckUserAllowed(ctx context.Context, userID int64) error {
	if userID != 0 && loginhelper.IsSuperAdmin(userID) {
		return errs.New(0, "不允许操作超级管理员用户", "")
	}
	return nil
}

// CheckUserDataScope 校验当前用户对给定用户是否有数据权限（对应 Java checkUserDataScope）。
// 超管短路放行；非超管按主键统计可见用户数，为 0 即越权。
//
// Java 的 countUserById 带 @DataPermission 注入隔离条件，Go 侧数据权限尚未落地，
// 此处只按主键计数——等数据权限落地后挂上过滤，调用点不必改。
func (s *UserService) CheckUserDataScope(ctx context.Context, userID int64) error {
	if userID == 0 {
		return nil
	}
	if loginhelper.IsSuperAdmin(loginhelperFromCtx(ctx)) {
		return nil
	}
	count, err := repository.NewUserRepository(database.DB()).CountByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if count == 0 {
		return errs.New(0, "没有权限访问用户数据！", "")
	}
	return nil
}

// InsertUser 新增用户（对应 Java insertUser）。
// 校验部门数据权限 + 账号/手机/邮箱唯一（手机/邮箱非空时）后，
// BCrypt 密码、发雪花主键、写主行与角色/岗位关联。唯一冲突返回对应哨兵错误。
func (s *UserService) InsertUser(ctx context.Context, b *bo.SysUserBo) error {
	if b == nil {
		return errors.New("service: 用户入参为空")
	}
	if err := DeptSvcApp.CheckDeptDataScope(ctx, loginhelperFromCtx(ctx), b.DeptID); err != nil {
		return err
	}
	if unique, err := s.CheckUserNameUnique(ctx, b.UserName, b.UserID); err != nil {
		return err
	} else if !unique {
		return ErrUserNameExists
	}
	if b.PhoneNumber != "" {
		if unique, err := s.CheckPhoneUnique(ctx, b.PhoneNumber, b.UserID); err != nil {
			return err
		} else if !unique {
			return ErrUserPhoneExists
		}
	}
	if b.Email != "" {
		if unique, err := s.CheckEmailUnique(ctx, b.Email, b.UserID); err != nil {
			return err
		} else if !unique {
			return ErrUserEmailExists
		}
	}

	hashed, err := bcrypt.Hashpw(b.Password)
	if err != nil {
		return err
	}
	add := bo.Conv.ConvertToSysUser(b)
	add.UserID = snowflake.Next() // user_id 无 auto_increment
	add.Password = hashed
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	repo := repository.NewUserRepository(database.DB())
	if err := repo.Insert(ctx, add); err != nil {
		return err
	}
	b.UserID = add.UserID
	if err := s.insertUserRole(ctx, b.UserID, b.RoleIDs, false); err != nil {
		return err
	}
	return s.insertUserPost(ctx, b.UserID, b.PostIDs, false)
}

// UpdateUser 修改用户（对应 Java updateUser）。
// 校验操作权限 + 数据权限 + 部门数据权限 + 唯一性后，清旧重建角色/岗位，再更新主行。
// 主行未生效（flag<1）抛 ErrUserUpdateFailed；并失效该用户昵称缓存。
func (s *UserService) UpdateUser(ctx context.Context, b *bo.SysUserBo) error {
	if b == nil || b.UserID <= 0 {
		return errors.New("service: 用户主键不能为空")
	}
	if err := s.CheckUserAllowed(ctx, b.UserID); err != nil {
		return err
	}
	if err := s.CheckUserDataScope(ctx, b.UserID); err != nil {
		return err
	}
	if err := DeptSvcApp.CheckDeptDataScope(ctx, loginhelperFromCtx(ctx), b.DeptID); err != nil {
		return err
	}
	if unique, err := s.CheckUserNameUnique(ctx, b.UserName, b.UserID); err != nil {
		return err
	} else if !unique {
		return ErrUserNameExists
	}
	if b.PhoneNumber != "" {
		if unique, err := s.CheckPhoneUnique(ctx, b.PhoneNumber, b.UserID); err != nil {
			return err
		} else if !unique {
			return ErrUserPhoneExists
		}
	}
	if b.Email != "" {
		if unique, err := s.CheckEmailUnique(ctx, b.Email, b.UserID); err != nil {
			return err
		} else if !unique {
			return ErrUserEmailExists
		}
	}

	// 先清旧重建关联再改主行：与 Java 同序。此处无事务包裹，主行失败则关联已重建，
	// 重试即可收敛——反向（先主行后关联）一旦关联失败会留下"主行改了、关联没动"的脏状态。
	if err := s.insertUserRole(ctx, b.UserID, b.RoleIDs, true); err != nil {
		return err
	}
	if err := s.insertUserPost(ctx, b.UserID, b.PostIDs, true); err != nil {
		return err
	}
	rows, err := repository.NewUserRepository(database.DB()).
		UpdateByID(ctx, b.UserID, buildUserUpdateColumns(b))
	if err != nil {
		return err
	}
	if rows < 1 {
		return ErrUserUpdateFailed
	}
	// 对应 Java @CacheEvict(cacheNames = SYS_NICKNAME, key = userId)。
	_ = cache.Evict(ctx, constant.CacheSysNickname, strconv.FormatInt(b.UserID, 10))
	return nil
}

// buildUserUpdateColumns 组装修改用户的更新列。
// password 不在其内：改密走 resetPwd 专用路径，编辑表单不该顺带刷密码。
// user_type/status 缺省即视为不改：漏传不该把线上的值刷成空串。
// 其余表单字段一律写入，让前端能把邮箱/备注清空——这正是编辑表单的本意，
// 故不能用 Updates(struct)（它跳过零值，空串会被当成未修改而丢弃）。
func buildUserUpdateColumns(b *bo.SysUserBo) map[string]any {
	columns := map[string]any{
		"dept_id":      b.DeptID,
		"user_name":    b.UserName,
		"nick_name":    b.NickName,
		"email":        b.Email,
		"phone_number": b.PhoneNumber,
		"gender":       b.Gender,
		"remark":       b.Remark,
	}
	if b.UserType != "" {
		columns["user_type"] = b.UserType
	}
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// DeleteUserByIDs 批量删除用户（对应 Java deleteUserByIds）。
// 当前登录用户不得在删除集合内；逐个校验操作权限与数据权限后，整批清关联再删主行。
// 主行未生效（flag<1）抛 ErrUserDeleteFailed。
func (s *UserService) DeleteUserByIDs(ctx context.Context, currentUserID int64,
	ids []int64) error {

	if len(ids) == 0 {
		return errors.New("service: 用户主键不能为空")
	}
	if containsInt64(ids, currentUserID) {
		return errs.New(0, "当前用户不能删除", "")
	}
	for _, id := range ids {
		if err := s.CheckUserAllowed(ctx, id); err != nil {
			return err
		}
		if err := s.CheckUserDataScope(ctx, id); err != nil {
			return err
		}
	}

	repo := repository.NewUserRepository(database.DB())
	if _, err := repo.DeleteUserRolesByUserIDs(ctx, ids); err != nil {
		return err
	}
	if _, err := repo.DeleteUserPostsByUserIDs(ctx, ids); err != nil {
		return err
	}
	rows, err := repo.DeleteByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if rows < 1 {
		return ErrUserDeleteFailed
	}
	return nil
}

// SelectByIDs 按用户ID串/部门取基础信息（对应 Java selectUserByIds），供 optionselect 用。
func (s *UserService) SelectByIDs(ctx context.Context, userIds []int64,
	deptID int64) ([]*vo.SysUserVo, error) {

	rows, err := repository.NewUserRepository(database.DB()).SelectByIDs(ctx, userIds, deptID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysUserVoList(rows), nil
}

// ResetUserPwdWithCheck 重置密码（对应 Java resetPwd controller 分支）。
// 校验操作权限与数据权限后复用 ResetUserPwd（内部 BCrypt）。
func (s *UserService) ResetUserPwdWithCheck(ctx context.Context, b *bo.SysUserBo) error {
	if b == nil || b.UserID <= 0 {
		return errors.New("service: 用户主键不能为空")
	}
	if err := s.CheckUserAllowed(ctx, b.UserID); err != nil {
		return err
	}
	if err := s.CheckUserDataScope(ctx, b.UserID); err != nil {
		return err
	}
	return s.ResetUserPwd(ctx, b.UserID, b.Password)
}

// UpdateUserStatusWithCheck 修改用户状态（对应 Java changeStatus controller 分支）。
// 校验操作权限与数据权限后更新状态；主行未生效返回 ErrUserNotFound（对齐 toAjax(0) 失败口径）。
func (s *UserService) UpdateUserStatusWithCheck(ctx context.Context,
	b *bo.SysUserBo) error {

	if b == nil || b.UserID <= 0 {
		return errors.New("service: 用户主键不能为空")
	}
	if err := s.CheckUserAllowed(ctx, b.UserID); err != nil {
		return err
	}
	if err := s.CheckUserDataScope(ctx, b.UserID); err != nil {
		return err
	}
	rows, err := repository.NewUserRepository(database.DB()).
		UpdateStatus(ctx, b.UserID, b.Status)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Unlock 解锁用户（对应 Java unlock）。
// 用户不存在抛错；存在则清掉 Redis 里的密码错误计数键，fail-open（Redis 故障不阻断）。
func (s *UserService) Unlock(ctx context.Context, userID int64) error {
	user, err := s.SelectUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errs.New(0, "用户不存在", "")
	}
	key := constant.PwdErrCntKeyPrefix + user.UserName
	if err := redis.Client().Del(ctx, key).Err(); err != nil {
		log.Printf("[user] 清除用户 %s 密码错误次数失败: %v", user.UserName, err)
	}
	return nil
}

// GetUserInfoByID 按用户编号取详情 + 角色ID + 岗位 + 可授权角色（对应 Java getInfo(userId)）。
// userID <= 0（根路径）时只返回可授权角色列表；非超管查看非超管用户时滤掉超管角色。
func (s *UserService) GetUserInfoByID(ctx context.Context,
	userID, currentUserID int64) (*vo.SysUserInfoVo, error) {

	info := &vo.SysUserInfoVo{}
	if userID > 0 {
		if err := s.CheckUserDataScope(ctx, userID); err != nil {
			return nil, err
		}
		user, err := s.SelectUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if user != nil {
			info.User = *user
			info.RoleIDs, err = RoleSvcApp.SelectRoleListByUserId(ctx, userID)
			if err != nil {
				return nil, err
			}
			if user.DeptID > 0 {
				posts, err := PostSvcApp.QueryList(ctx,
					bo.SysPostQueryBo{DeptID: user.DeptID}, 0)
				if err != nil {
					return nil, err
				}
				info.Posts = posts
				info.PostIDs, err = PostSvcApp.SelectPostListByUserId(ctx, userID)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	roles, err := RoleSvcApp.SelectRoleList(ctx, bo.SysRoleQueryBo{Status: constant.StatusNormal})
	if err != nil {
		return nil, err
	}
	info.Roles = filterSuperAdminRoles(roles, userID)
	return info, nil
}

// AuthRole 取用户授权角色信息（对应 Java authRole）。
// 校验数据权限后回填用户与其可授权角色（带 flag 标记已授权），非超管查看非超管用户时滤掉超管角色。
func (s *UserService) AuthRole(ctx context.Context,
	userID int64) (*vo.SysUserInfoVo, error) {

	if err := s.CheckUserDataScope(ctx, userID); err != nil {
		return nil, err
	}
	user, err := s.SelectUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := RoleSvcApp.SelectRolesAuthByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &vo.SysUserInfoVo{
		User:  derefUserVo(user),
		Roles: filterSuperAdminRoles(roles, userID),
	}, nil
}

// InsertUserAuth 用户授权角色（对应 Java insertUserAuth）。
// 校验数据权限后清旧重建该用户的角色关联。
func (s *UserService) InsertUserAuth(ctx context.Context, userID int64,
	roleIDs []int64) error {

	if err := s.CheckUserDataScope(ctx, userID); err != nil {
		return err
	}
	return s.insertUserRole(ctx, userID, roleIDs, true)
}

// DeptTree 取用户筛选用的部门树（对应 Java deptTree），透传部门 service。
func (s *UserService) DeptTree(ctx context.Context,
	q bo.SysDeptQueryBo) ([]*tree.Tree[int64], error) {

	return DeptSvcApp.SelectDeptTreeList(ctx, q)
}

// SelectListByDept 取部门下全部用户（对应 Java selectUserListByDept）。
func (s *UserService) SelectListByDept(ctx context.Context,
	deptID int64) ([]*vo.SysUserVo, error) {

	rows, err := repository.NewUserRepository(database.DB()).SelectListByDept(ctx, deptID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysUserVoList(rows), nil
}

// insertUserRole 写用户-角色关联（对应 Java insertUserRole）。
// 空集直接返回且不清旧：导入更新场景（已有用户、模板无角色列）需保留既有角色，
// 与 Java ArrayUtil.isEmpty(roleIds) 在清旧之前 return 一致。
// 非目标超管用户剔除超管角色；空集（剔除后）抛错；数据权限校验可见角色数；
// clear=true 时先清旧再批量插。
func (s *UserService) insertUserRole(ctx context.Context, userID int64,
	roleIDs []int64, clear bool) error {

	if len(roleIDs) == 0 {
		return nil
	}
	if !loginhelper.IsSuperAdmin(userID) {
		filtered := make([]int64, 0, len(roleIDs))
		for _, rid := range roleIDs {
			if rid != constant.SuperAdminRoleID {
				filtered = append(filtered, rid)
			}
		}
		roleIDs = filtered
	}
	if len(roleIDs) == 0 {
		return errs.New(0, "不允许为普通用户分配超级管理员角色，请至少选择一个其他角色", "")
	}
	// 数据权限校验：可见角色数与传入数不等即越权（对应 Java selectRoleCount）。
	count, err := repository.NewRoleRepository(database.DB()).CountByRoleIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	if count != int64(len(roleIDs)) {
		return errs.New(0, "没有权限访问角色的数据", "")
	}
	repo := repository.NewUserRepository(database.DB())
	if clear {
		if _, err := repo.DeleteUserRolesByUserID(ctx, userID); err != nil {
			return err
		}
	}
	return repo.InsertUserRoles(ctx, userID, roleIDs)
}

// insertUserPost 写用户-岗位关联（对应 Java insertUserPost）。
// 空集直接返回且不清旧（同 insertUserRole 的语义，保留既有岗位）。
// 数据权限校验可见岗位数；clear=true 时先清旧再批量插。
func (s *UserService) insertUserPost(ctx context.Context, userID int64,
	postIDs []int64, clear bool) error {

	if len(postIDs) == 0 {
		return nil
	}
	count, err := repository.NewPostRepository(database.DB()).CountByIDs(ctx, postIDs)
	if err != nil {
		return err
	}
	if count != int64(len(postIDs)) {
		return errs.New(0, "没有权限访问岗位的数据", "")
	}
	repo := repository.NewUserRepository(database.DB())
	if clear {
		if _, err := repo.DeleteUserPostsByUserID(ctx, userID); err != nil {
			return err
		}
	}
	return repo.InsertUserPosts(ctx, userID, postIDs)
}

// filterSuperAdminRoles 过滤掉超管角色，对齐 Java StreamUtils.filter(r -> !r.isSuperAdmin())。
// 目标用户本身是超管时不滤（与 Java LoginHelper.isSuperAdmin(userId) ? roles : filter 一致）。
func filterSuperAdminRoles(roles []*vo.SysRoleVo, userID int64) []*vo.SysRoleVo {
	if loginhelper.IsSuperAdmin(userID) {
		return roles
	}
	out := make([]*vo.SysRoleVo, 0, len(roles))
	for _, r := range roles {
		if r != nil && r.RoleID == constant.SuperAdminRoleID {
			continue
		}
		out = append(out, r)
	}
	return out
}

// derefUserVo 解引用用户 VO，nil 时返回零值（对齐 Java R.ok(null) 的 data: null 语义）。
func derefUserVo(u *vo.SysUserVo) vo.SysUserVo {
	if u == nil {
		return vo.SysUserVo{}
	}
	return *u
}
