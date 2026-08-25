package service

import (
	"context"
	"errors"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/pkg/auth"
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

// BuildLoginUser 构造登录用户（对应 Java SysLoginService#buildLoginUser）。
// user/dept/权限等由 system 各 service 填充；client 字段（ClientKey/DeviceType）来自授权客户端。
// 不在此设 IPAddr——IP 由 handler 注入 context，afterLogin 时按需取（对应 Java 在 record 阶段 getClientIP）。
func (*SysLoginService) BuildLoginUser(user *systemmodel.SysUser,
	client *systemvo.SysClientVo) *auth.LoginUser {

	userType := user.UserType
	if userType == "" {
		userType = auth.UserTypeSys
	}

	return &auth.LoginUser{
		UserID:   user.UserID,
		DeptID:   user.DeptID,
		Username: user.UserName,
		Nickname: user.NickName,
		UserType: userType,

		// TODO(阶段 2): DeptName / DeptCategory / 权限 / 角色需 sys_dept / sys_menu / sys_role / sys_post。
		LoginTime: time.Now().UnixMilli(),
		// IPAddr / LoginLocation / Browser / OS 在 afterLogin 按请求上下文与 UA 填充（阶段 3）。

		ClientKey:  client.ClientKey,
		DeviceType: client.DeviceType,
	}
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
