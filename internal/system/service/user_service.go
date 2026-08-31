package service

import (
	"context"
	"errors"
	"log"

	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
)

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
