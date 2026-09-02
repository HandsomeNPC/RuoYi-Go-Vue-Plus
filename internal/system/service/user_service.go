package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/bcrypt"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
)

// ErrUserPhoneExists 手机号已被占用。
var ErrUserPhoneExists = errors.New("service: 手机号已存在")

// ErrUserEmailExists 邮箱已被占用。
var ErrUserEmailExists = errors.New("service: 邮箱已存在")

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
