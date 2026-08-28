package service

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/ip"
	authmodel "ruoyi-go-vue-plus/pkg/model"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/useragent"
)

// SysLoginService 登录态组装（对应 Java SysLoginService）。
type SysLoginService struct{}

// SysLoginSvcApp 包级实例。
var SysLoginSvcApp = new(SysLoginService)

// BuildLoginUser 组装登录态上下文并填充终端信息(IP/位置/浏览器/OS)。
func (*SysLoginService) BuildLoginUser(req *http.Request, user *systemvo.SysUserVo) (*authmodel.LoginUser, error) {
	ctx := req.Context()
	loginUser := &authmodel.LoginUser{
		UserID:   user.UserID,
		DeptID:   user.DeptID,
		Username: user.UserName,
		Nickname: user.NickName,
		UserType: user.UserType,
	}

	// 终端信息
	if loginUser.IPAddr == "" {
		loginUser.IPAddr = ip.ClientIP(req)
	}
	if loginUser.LoginLocation == "" && loginUser.IPAddr != "" {
		loginUser.LoginLocation = ip.RealAddressByIP(loginUser.IPAddr)
	}
	if loginUser.Browser == "" || loginUser.OS == "" {
		browser, osName := useragent.Parse(req.Header.Get("User-Agent"))
		if loginUser.Browser == "" {
			loginUser.Browser = browser
		}
		if loginUser.OS == "" {
			loginUser.OS = osName
		}
	}

	// 并发拉取部门/权限/角色/岗位，互不竞争，写入 loginUser 不同字段。
	g, gctx := errgroup.WithContext(ctx)
	// 部门名 / 部门类别：deptId 为空则留空。
	g.Go(func() error {
		if user.DeptID == 0 {
			return nil
		}
		dept, err := systemservice.DeptSvcApp.SelectByID(gctx, user.DeptID)
		if err != nil {
			return err
		}
		if dept != nil {
			loginUser.DeptName = dept.DeptName
			loginUser.DeptCategory = dept.DeptCategory
		}
		return nil
	})
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
		// 角色摘要，供 Roles 与 DataScopeRoleMap 共用。
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
	// 读取登录错误次数，默认 0。
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
		errorNumber++
		if err := rdb.Set(ctx, key, errorNumber,
			time.Duration(lockMinutes)*time.Minute).Err(); err != nil {
			log.Printf("[auth] 写入密码错误次数失败: %v", err)
		}
		if errorNumber >= maxRetry {
			// 达上限则锁定。
			// TODO(阶段 3): recordLoginInfo(username, LOGIN_FAIL, RetryExceedMsg)。
			return errs.New(0, loginType.RetryExceedMsg(ctx, maxRetry, lockMinutes), "")
		}
		// TODO(阶段 3): recordLoginInfo(username, LOGIN_FAIL, RetryCountMsg)。
		return errs.New(0, loginType.RetryCountMsg(ctx, errorNumber), "")
	}

	// 登录成功，清空错误次数。
	if err := rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[auth] 清除密码错误次数失败: %v", err)
	}
	return nil
}
