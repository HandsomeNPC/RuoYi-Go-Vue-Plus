package service

import (
	"context"
	"errors"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/redis"
)

// SysLoginService 登录态组装（对应 Java SysLoginService）。
type SysLoginService struct{}

// SysLoginSvcApp 包级实例。
var SysLoginSvcApp = new(SysLoginService)

// BuildLoginUser 根据用户视图对象组装登录态上下文
func (*SysLoginService) BuildLoginUser(ctx context.Context, user *systemvo.SysUserVo) (*authmodel.LoginUser, error) {
	loginUser := &authmodel.LoginUser{
		UserID:   user.UserID,
		DeptID:   user.DeptID,
		Username: user.UserName,
		Nickname: user.NickName,
		UserType: user.UserType,
	}

	// 部门名 / 部门类别：deptId 为空则留空；查一次 dept 复用取两个字段（对应 Java Opt.map(deptService::selectDeptById)）。
	if user.DeptID != 0 {
		dept, err := systemservice.DeptSvcApp.SelectByID(ctx, user.DeptID)
		if err != nil {
			return nil, err
		}
		if dept != nil {
			loginUser.DeptName = dept.DeptName
			loginUser.DeptCategory = dept.DeptCategory
		}
	}

	// 四组查询并发拉取（对应 Java ThreadUtils.virtualInvokeAll），写入 loginUser 不同字段，互不竞争。
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		mp, err := systemservice.PermissionSvcApp.GetMenuPermission(gctx, user.UserID)
		if err != nil {
			return err
		}
		loginUser.MenuPermission = mp
		return nil
	})
	g.Go(func() error {
		rp, err := systemservice.PermissionSvcApp.GetRolePermission(gctx, user.UserID)
		if err != nil {
			return err
		}
		loginUser.RolePermission = rp
		return nil
	})
	g.Go(func() error {
		roles, err := systemservice.RoleSvcApp.SelectRolesByUserId(gctx, user.UserID)
		if err != nil {
			return err
		}
		// 角色摘要（对应 Java BeanUtil.copyToList(roles, RoleDTO.class)），
		// 供 loginUser.Roles 与 GetDataScopeRoleMap 共用，避免重复转换。
		roleDTOs := systemdto.Conv.ConvertToRoleDTOList(roles)
		loginUser.Roles = roleDTOs
		loginUser.DataScopeRoleMap, err = systemservice.PermissionSvcApp.GetDataScopeRoleMap(gctx, roleDTOs)
		return err
	})
	g.Go(func() error {
		posts, err := systemservice.PostSvcApp.SelectPostsByUserId(gctx, user.UserID)
		if err != nil {
			return err
		}
		loginUser.Posts = systemdto.Conv.ConvertToPostDTOList(posts)
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return loginUser, nil
}

// CheckLogin 执行登录失败次数校验，并在成功后清空失败计数
func (*SysLoginService) CheckLogin(ctx context.Context, loginType enum.LoginType,
	username string, authSuccess func() bool) error {

	key := constant.PwdErrCntKeyPrefix + username
	passwdCfg := config.Get().User.Password
	maxRetry := passwdCfg.MaxRetryCount
	lockMinutes := passwdCfg.LockTime

	rdb := redis.Client()
	// 获取用户登录错误次数，默认为 0（可自定义限制策略，例如: key + username + ip）。
	errorNumber, err := rdb.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		log.Printf("[auth] 读取密码错误次数失败(按 0 次继续): %v", err)
		errorNumber = 0
	}
	// 锁定时间内登录则踢出。
	if errorNumber >= maxRetry {
		// TODO(阶段 3): recordLoginInfo(username, LOGIN_FAIL, RetryExceedMsg)。
		return errs.New(0, loginType.RetryExceedMsg(ctx, maxRetry, lockMinutes), "")
	}

	if !authSuccess() {
		// 错误次数递增。
		errorNumber++
		if err := rdb.Set(ctx, key, errorNumber,
			time.Duration(lockMinutes)*time.Minute).Err(); err != nil {
			log.Printf("[auth] 写入密码错误次数失败: %v", err)
		}
		if errorNumber >= maxRetry {
			// 达到规定错误次数则锁定登录。
			// TODO(阶段 3): recordLoginInfo(username, LOGIN_FAIL, RetryExceedMsg)。
			return errs.New(0, loginType.RetryExceedMsg(ctx, maxRetry, lockMinutes), "")
		}
		// 未达到规定错误次数。
		// TODO(阶段 3): recordLoginInfo(username, LOGIN_FAIL, RetryCountMsg)。
		return errs.New(0, loginType.RetryCountMsg(ctx, errorNumber), "")
	}

	// 登录成功，清空错误次数。
	if err := rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[auth] 清除密码错误次数失败: %v", err)
	}
	return nil
}
