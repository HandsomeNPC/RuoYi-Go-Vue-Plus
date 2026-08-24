// Package service system 模块业务逻辑层。
//
// 本层导出的 Service 供 auth 模块以 in-process 方式复用(同进程函数调用，无网络开销)。
package service

import (
	"context"
	"errors"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// ErrUserNotFound 用户不存在，转自 repository 层的同名错误。
//
// 在本层重新导出而非让调用方 import repository：auth 模块只依赖
// system 的 **service** 层（CLAUDE.md 的模块隔离约定），
// 不该为了判断一个错误而穿透到数据访问层。
var ErrUserNotFound = errors.New("service: 用户不存在")

// UserService 用户业务逻辑。
//
// 空结构体、无状态：db 走包级 database.DB()，不在本层持有 ——
// 与 handler 层的 AuthApi 同一套做法。
//
// 导出给 internal/auth 复用 —— 这是 M1 的架构验证点：
// auth 进程直接 import 本包、同进程函数调用，不走 HTTP。
type UserService struct{}

// UserSvcApp 包级实例，auth 模块 in-process 复用（service.UserSvcApp.xxx）。
var UserSvcApp = new(UserService)

// GetByUsername 按账号查用户，不存在时返回 ErrUserNotFound。
//
// **给 auth 模块的登录流程用**，返回的实体带 BCrypt 哈希（Password 字段）——
// 密码比对必须在调用方做，本层不碰密码校验：那属于认证逻辑，
// 归 internal/auth/service（对齐 Java 侧 PasswordAuthStrategy 做比对、
// SysUserMapper 只管查）。
//
// 账号状态（status=1 停用）**也不在这里判**，同样归调用方：
// 「停用的用户不能登录」是认证规则，而本方法将来还要服务于
// 用户管理接口（阶段 2），那里查停用用户是正常需求。
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

// UpdateLoginInfo 更新最后登录 IP 与时间，对应 Java 的 updateLastLoginInfo。
//
// 登录成功后的副作用之一，由 auth 模块在签发 token 后调用。
func (s *UserService) UpdateLoginInfo(ctx context.Context, userID int64, ip string) error {
	return repository.NewUserRepository(database.DB()).UpdateLoginInfo(ctx, userID, ip)
}
