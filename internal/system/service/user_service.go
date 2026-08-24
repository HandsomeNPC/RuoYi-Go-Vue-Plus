package service

import (
	"context"
	"errors"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("service: 用户不存在")

// UserService 用户业务逻辑。
type UserService struct{}

// UserSvcApp 包级实例。
var UserSvcApp = new(UserService)

// GetByUsername 按账号查用户，不存在时返回 ErrUserNotFound。
func (s *UserService) GetByUsername(ctx context.Context, username string) (*model.SysUser, error) {
	user, err := repository.NewUserRepository(database.DB()).SelectByUserName(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateLoginInfo 更新最后登录 IP 与时间。
func (s *UserService) UpdateLoginInfo(ctx context.Context, userID int64, ip string) error {
	return repository.NewUserRepository(database.DB()).UpdateLoginInfo(ctx, userID, ip)
}
