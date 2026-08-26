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
