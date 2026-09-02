package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"golang.org/x/sync/errgroup"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/ip"
	authmodel "ruoyi-go-vue-plus/pkg/model"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
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

// RecordOnlineUser 把当前会话的在线摘要写入 Redis(online_tokens:<token>)，
// 对应 Java UserLoginSuccessListener.handleLoginSuccess 写 UserOnlineDTO。
//
// 必须在 loginhelper.Login 返回后调：EventLogin 在 Login 内部即触发，彼时
// token-session 尚未写入 LoginUser，监听器取不到终端信息，故写入只能落在 auth 层。
// TTL 镜像 token 自身剩余 TTL：令牌失效即在线摘要失效，与 Java 按 loginParameter.timeout 设值等价。
// 用 context.Background 而非请求 ctx：缓存写不该绑请求生命周期，与 Java 走 SpringUtils 事件一致。
func (*SysLoginService) RecordOnlineUser(lu *authmodel.LoginUser, token string) {
	if lu == nil || token == "" {
		return
	}
	dto := systemdto.UserOnlineDTO{
		TokenID:       token,
		DeptName:      lu.DeptName,
		UserName:      lu.Username,
		ClientKey:     lu.ClientKey,
		DeviceType:    lu.DeviceType,
		IPAddr:        lu.IPAddr,
		LoginLocation: lu.LoginLocation,
		Browser:       lu.Browser,
		OS:            lu.OS,
		LoginTime:     time.Now().UnixMilli(),
	}
	payload, err := json.Marshal(dto)
	if err != nil {
		log.Printf("[auth] 序列化在线用户摘要失败: %v", err)
		return
	}
	key := constant.OnlineTokenKeyPrefix + token
	// 取 token 剩余 TTL 镜像设值：-1 永不过期则不设 TTL，-2/0 以下视为令牌已失效，不写。
	ttl, err := sagin.GetManager().GetTokenTimeout(token)
	if err != nil {
		log.Printf("[auth] 读取 token TTL 失败(按不设 TTL 写入): %v", err)
	}
	var expiration time.Duration
	switch {
	case ttl > 0:
		expiration = time.Duration(ttl) * time.Second
	case ttl == -1:
		// 永不过期
	default:
		return
	}
	if err := redis.Client().Set(context.Background(), key, payload, expiration).Err(); err != nil {
		log.Printf("[auth] 写入在线用户摘要 %s 失败: %v", key, err)
	}
}

// Logout 退出登录，对应 Java SysLoginService.logout。
//
// token 由 handler 从请求上下文取（sagin.GetTokenFromCtx），service 层不碰 *gin.Context。
//
// **不返回 error**：对照 Java 的 try/finally——取登录态与注销都把 NotLoginException 吞掉，
// 退出接口对外恒为成功。这里同样只记日志：token 已失效/已登出时 LogoutByToken 会返回
// ErrInvalidTokenData，等价于 Java 的 NotLoginException，不该让前端登出失败。
//
// 顺序要紧：必须先取 LoginUser 再注销。注销会删掉 token-session，之后就拿不到用户名了。
func (s *SysLoginService) Logout(req *http.Request, token string) {
	if token == "" {
		// 未携带 token，等同 Java loginUser 为 null 直接 return。
		return
	}
	if loginUser := loginhelper.GetLoginUserByToken(token); loginUser != nil {
		ctx := context.Background()
		if req != nil {
			ctx = req.Context()
		}
		s.RecordLoginInfo(req, loginUser.Username, constant.ConstantLogout,
			i18n.Msg(ctx, "user.logout.success"))
	}
	// 对照 Java finally 里的 StpUtil.logout()：删 token 信息、账号映射、token-session。
	if err := sagin.LogoutByToken(token); err != nil {
		log.Printf("[auth] 注销 token 失败(按已登出处理): %v", err)
	}
}

// RecordLoginInfo 记录登录信息，对应 Java SysLoginService.recordLoginInfo。
//
// Java 从 ServletUtils.getRequest()（线程绑定）取 IP/UA/clientid 后发 Spring 事件；
// Go 无线程本地，req 必须由调用方显式传入，req 为 nil 时只记 username/status/message。
//
// 落库异步，本函数立即返回、不报错：日志写失败不该影响登录结果。ctx 用
// context.WithoutCancel 脱开请求生命周期，否则响应一发完 ctx 即取消，落库必失败。
func (*SysLoginService) RecordLoginInfo(req *http.Request, username, status, message string) {
	evt := &systemdto.LoginInfoEvent{
		Username: username,
		Status:   status,
		Message:  message,
	}
	ctx := context.Background()
	if req != nil {
		evt.IP = ip.ClientIP(req)
		evt.UserAgent = req.Header.Get("User-Agent")
		evt.ClientID = req.Header.Get(constant.ClientIDHeader)
		ctx = context.WithoutCancel(req.Context())
	}
	// 同进程直接调 system service，不走 HTTP（对照 CLAUDE.md 的 in-process 约定）。
	systemservice.LoginInfoSvcApp.RecordLoginInfo(ctx, evt)
}

// CheckLogin 执行登录失败次数校验，并在成功后清空失败计数。
// req 除了提供 ctx，还用于记录登录日志（取 IP/UA/clientid），对照 BuildLoginUser 的取参方式。
func (s *SysLoginService) CheckLogin(req *http.Request, loginType enum.LoginType,
	username string, authSuccess func() bool) error {

	ctx := context.Background()
	if req != nil {
		ctx = req.Context()
	}

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
		msg := loginType.RetryExceedMsg(ctx, maxRetry, lockMinutes)
		s.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
		return errs.New(0, msg, "")
	}

	if !authSuccess() {
		errorNumber++
		if err := rdb.Set(ctx, key, errorNumber,
			time.Duration(lockMinutes)*time.Minute).Err(); err != nil {
			log.Printf("[auth] 写入密码错误次数失败: %v", err)
		}
		if errorNumber >= maxRetry {
			// 达上限则锁定。
			msg := loginType.RetryExceedMsg(ctx, maxRetry, lockMinutes)
			s.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
			return errs.New(0, msg, "")
		}
		msg := loginType.RetryCountMsg(ctx, errorNumber)
		s.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
		return errs.New(0, msg, "")
	}

	// 登录成功，清空错误次数。
	if err := rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[auth] 清除密码错误次数失败: %v", err)
	}
	return nil
}
