package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/redis"
)

// AuthService 认证业务逻辑：登录、登出。
type AuthService struct{}

// AuthSvcApp 包级实例，handler 直接 service.AuthSvcApp.Login / .Logout 调用。
var AuthSvcApp = new(AuthService)

// loginStrategy 一种登录方式的凭证校验
type loginStrategy func(ctx context.Context, body *authmodel.LoginBody,
	client *systemmodel.SysClient) (*systemmodel.SysUser, error)

// strategies 按 grantType 分派登录策略。
var strategies = map[string]loginStrategy{
	enum.LoginTypePassword.Code: AuthSvcApp.passwordLogin,
}

// Login 登录
func (s *AuthService) Login(ctx context.Context, body *authmodel.LoginBody, clientIP string) (*authmodel.LoginVo, error) {
	// 1. 查客户端并校验。
	client, err := systemservice.ClientSvcApp.GetByClientID(ctx, body.ClientID)
	if err != nil {
		if errors.Is(err, systemservice.ErrClientNotFound) {
			log.Printf("[auth] 客户端id: %s 认证类型: %s 异常", body.ClientID, body.GrantType)
			return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
		}
		return nil, err
	}
	if !client.SupportsGrantType(body.GrantType) {
		log.Printf("[auth] 客户端id: %s 不支持认证类型: %s", body.ClientID, body.GrantType)
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
	}
	if client.Status != constant.StatusNormal {
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.blocked"), "")
	}

	// 2. 分派策略校验凭证。
	strategy, ok := strategies[body.GrantType]
	if !ok {
		// 走到这里说明 sys_client.grant_type 里配了一个还没实现的类型
		// （如阶段 4 才做的 sms）。对齐 Java 的 ServiceException("授权类型不正确!")。
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
	}
	user, err := strategy(ctx, body, client)
	if err != nil {
		return nil, err
	}

	// 3. 构造登录用户。
	loginUser := s.buildLoginUser(user, client, clientIP)

	// 4. 签发 token 并写会话。
	vo, err := s.issue(ctx, loginUser, client)
	if err != nil {
		return nil, err
	}

	// 5. 登录成功的副作用。失败只打日志不影响登录结果（详见该方法）。
	s.afterLogin(ctx, loginUser, client, vo.AccessToken)

	return vo, nil
}

// passwordLogin 密码登录
func (s *AuthService) passwordLogin(ctx context.Context, body *authmodel.LoginBody,
	_ *systemmodel.SysClient) (*systemmodel.SysUser, error) {

	// TODO: 验证码校验。原项目 captcha.enable 默认 false
	// （application.yml:20），且验证码生成接口属阶段 3 —— 现在校验会让
	// 谁都登不进来。body.Code / body.UUID 字段已就位。

	// 1. 查用户并校验状态。
	user, err := systemservice.UserSvcApp.GetByUsername(ctx, body.Username)
	if err != nil {
		if errors.Is(err, systemservice.ErrUserNotFound) {
			log.Printf("[auth] 登录用户: %s 不存在", body.Username)
			return nil, errs.New(0, i18n.Msg(ctx, "user.not.exists", body.Username), "")
		}
		return nil, err
	}
	if user.Status == enum.UserStatusDisable.Code {
		log.Printf("[auth] 登录用户: %s 已被停用", body.Username)
		return nil, errs.New(0, i18n.Msg(ctx, "user.blocked", body.Username), "")
	}

	// 2 + 3. 错误次数检查与密码比对。
	if err := s.checkPassword(ctx, body.Username, body.Password, user.Password); err != nil {
		return nil, err
	}
	return user, nil
}

// checkPassword 校验密码并维护错误次数
func (s *AuthService) checkPassword(ctx context.Context, username, password, hashed string) error {
	key := constant.PwdErrCntKeyPrefix + username
	passwdCfg := config.Get().User.Password
	maxRetry := passwdCfg.MaxRetryCount
	lockMinutes := passwdCfg.LockTime

	rdb := redis.Client()
	// 前置检查：已达上限则直接拒绝，不再比对密码。
	errCount, err := rdb.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		// Redis 故障不该让登录直接不可用，但也不能静默跳过限制 ——
		// 那等于在 Redis 挂掉时打开暴力破解的门。折中：当作 0 次继续，
		// 并打 error 级日志。密码本身仍然要比对，所以不是「放行」。
		log.Printf("[auth] 读取密码错误次数失败(按 0 次继续): %v", err)
		errCount = 0
	}
	if errCount >= maxRetry {
		return errs.New(0, i18n.Msg(ctx, enum.LoginTypePassword.RetryExceedKey, maxRetry, lockMinutes), "")
	}

	// 密码比对。
	verifyErr := auth.VerifyPassword(password, hashed)
	if verifyErr == nil {
		// 成功：清零计数。
		if err := rdb.Del(ctx, key).Err(); err != nil {
			log.Printf("[auth] 清除密码错误次数失败: %v", err)
		}
		return nil
	}
	if !errors.Is(verifyErr, auth.ErrPasswordMismatch) {
		// 哈希串格式非法（如迁移时漏加密的明文密码）—— 这是**数据问题**，
		// 不该计入用户的密码错误次数，否则会掩盖真正的原因。
		log.Printf("[auth] 用户 %s 的密码哈希格式非法: %v", username, verifyErr)
		return fmt.Errorf("密码校验失败: %w", verifyErr)
	}

	// 失败：递增计数并重设 TTL。
	errCount++
	if err := rdb.Set(ctx, key, errCount,
		time.Duration(lockMinutes)*time.Minute).Err(); err != nil {
		log.Printf("[auth] 写入密码错误次数失败: %v", err)
	}

	if errCount >= maxRetry {
		return errs.New(0, i18n.Msg(ctx, enum.LoginTypePassword.RetryExceedKey, maxRetry, lockMinutes), "")
	}
	return errs.New(0, i18n.Msg(ctx, enum.LoginTypePassword.RetryCountKey, errCount), "")
}

// buildLoginUser 构造登录用户，对应 Java SysLoginService.buildLoginUser（:155-182）。
//
// 阶段 1 只填得出基本信息。Java 侧那四个并行查询
// （menuPermission / rolePermission / roles+dataScopeRoleMap / posts）
// 依赖 sys_menu / sys_role / sys_post，**都是阶段 2 的表**；
// deptName / deptCategory 依赖 sys_dept，同样阶段 2。
//
// 字段留空而非省略，理由见 auth.LoginUser 的类型注释（存量会话反序列化）。
func (s *AuthService) buildLoginUser(user *systemmodel.SysUser,
	client *systemmodel.SysClient, clientIP string) *auth.LoginUser {

	userType := user.UserType
	if userType == "" {
		// 列默认值是 'sys_user'，但历史数据可能为空。
		// 空 userType 会让 LoginID() 返回 ok=false，登录直接失败 ——
		// 回落成后台用户，与列默认值一致。
		userType = auth.UserTypeSys
	}

	return &auth.LoginUser{
		UserID:   user.UserID,
		DeptID:   user.DeptID,
		Username: user.UserName,
		Nickname: user.NickName,
		UserType: userType,

		// TODO(阶段 2): DeptName / DeptCategory 需 sys_dept。
		// TODO(阶段 2): MenuPermission / RolePermission / Roles / Posts /
		// DataScopeRoleMap / RoleID 需 sys_menu / sys_role / sys_post。

		LoginTime: time.Now().UnixMilli(),
		IPAddr:    clientIP,
		// TODO(阶段 3): LoginLocation 需 IP 归属地库（Java 用 ip2region）。
		// Browser / OS 需解析 User-Agent，同阶段 3 的登录日志一并做。

		ClientKey:  client.ClientKey,
		DeviceType: client.DeviceType,
	}
}

// issue 签发 token 并写入会话。
//
// 两个超时的分工（对应 sys_client 的两列）：
//
//	Timeout(604800s=7d) -> JWT 的 exp，**绝对**有效期，到期必须重新登录
//	ActiveTimeout(1800s) -> Redis 会话 TTL，**滑动**空闲超时，每请求续期
//
// 前者签进 token 后即冻结，后者存在会话里（改配置对新会话立即生效），
// 详见 auth.Session 的说明。
func (s *AuthService) issue(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient) (*authmodel.LoginVo, error) {

	loginID, ok := loginUser.LoginID()
	if !ok {
		// userType 已在 buildLoginUser 里兜过底，走到这里说明 UserID 为 0，
		// 即库里有一条主键为 0 的用户 —— 数据问题，不是用户输入问题。
		return nil, fmt.Errorf("构造登录标识失败: userId=%d userType=%q",
			loginUser.UserID, loginUser.UserType)
	}

	ttl := time.Duration(client.Timeout) * time.Second

	claims := &auth.Claims{
		UserID:       loginUser.UserID,
		Username:     loginUser.Username,
		DeptID:       loginUser.DeptID,
		DeptName:     loginUser.DeptName,
		DeptCategory: loginUser.DeptCategory,

		// 客户端规则**在签发时冻结进 token**，鉴权时不查库 ——
		// 对齐 Java（SecurityConfig 从 token extra 读这三项）。
		// 代价是改了客户端规则要等存量 token 过期才生效。
		ClientID:          client.ClientID,
		ClientAccessPath:  client.AccessPath,
		ClientIPWhitelist: client.IPWhitelist,
	}
	claims.Subject = loginID

	token, err := auth.Sign(claims, config.Get().JWT.Secret, ttl)
	if err != nil {
		return nil, err
	}

	loginUser.Token = token
	loginUser.ExpireTime = time.Now().Add(ttl).UnixMilli()

	sess := &auth.Session{User: loginUser, ActiveTimeout: client.ActiveTimeout}
	if err := auth.NewSessionStore(redis.Client()).Save(ctx, token, sess); err != nil {
		return nil, err
	}

	return &authmodel.LoginVo{
		AccessToken: token,
		ExpireIn:    client.Timeout,
		ClientID:    client.ClientID,
	}, nil
}

// afterLogin 登录成功的副作用，对应 Java UserLoginSuccessListener（:41-69）。
//
// **全部失败只打日志、不影响登录结果**：token 已经签发、会话已经写入，
// 用户实际上已经登录成功了。此刻因为一条在线记录没写上就回失败，
// 会让前端拿不到 token 却又真的存在一个有效会话 —— 那是最难排查的状态。
//
// Java 侧靠 @Async 事件监听达到同样的解耦。Go 侧同步执行但吞掉错误：
// 这几个操作都很快，起 goroutine 反而要处理 ctx 已取消的问题
// （请求结束后 c.Request.Context() 就 done 了）。
func (s *AuthService) afterLogin(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient, token string) {

	// 在线用户记录，键 online_tokens:<token>，TTL 取客户端的绝对超时。
	// 阶段 3 的在线用户管理（查询、强退）读它。
	s.recordOnline(ctx, loginUser, client, token)

	// 更新 sys_user 的 login_ip / login_date。
	if err := systemservice.UserSvcApp.UpdateLoginInfo(ctx, loginUser.UserID, loginUser.IPAddr); err != nil {
		log.Printf("[auth] 更新用户 %d 最后登录信息失败: %v", loginUser.UserID, err)
	}

	// TODO(阶段 3): 登录日志落库（sys_login_info 表 + LoginInfoEvent 等价物）。
	// 现在只打日志 —— 表结构与 repository 属阶段 3。
	log.Printf("[auth] 用户 %s(%d) 登录成功, ip=%s, client=%s",
		loginUser.Username, loginUser.UserID, loginUser.IPAddr, client.ClientKey)
}

// recordOnline 写在线用户记录。
func (s *AuthService) recordOnline(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient, token string) {

	dto := authmodel.OnlineUser{
		TokenID:       token,
		UserName:      loginUser.Username,
		IPAddr:        loginUser.IPAddr,
		LoginLocation: loginUser.LoginLocation,
		Browser:       loginUser.Browser,
		OS:            loginUser.OS,
		DeptName:      loginUser.DeptName,
		ClientKey:     loginUser.ClientKey,
		DeviceType:    loginUser.DeviceType,
		LoginTime:     loginUser.LoginTime,
	}

	payload, err := json.Marshal(dto)
	if err != nil {
		log.Printf("[auth] 序列化在线用户记录失败: %v", err)
		return
	}

	var ttl time.Duration // 0 即不设过期
	if client.Timeout > 0 {
		ttl = time.Duration(client.Timeout) * time.Second
	}
	if err := redis.Client().Set(ctx, constant.OnlineTokenKeyPrefix+token, payload, ttl).Err(); err != nil {
		log.Printf("[auth] 写入在线用户记录失败: %v", err)
	}
}

// Logout 登出，对应原项目 SysLoginService.logout（:112-126）。
//
// 删会话是 JWT 唯一的作废手段 —— token 本身签发后不可撤销，
// 后续请求会在鉴权第 2 步（查会话）被拦下。
//
// **幂等**：token 为空、会话已不存在、重复登出都不报错。
// 对齐 Java 侧那两个 `catch (NotLoginException ignored)` ——
// 登出失败没有任何可行的补救动作，让前端拿到一个错误只会让它不敢清本地
// 状态，反而卡在一个「以为还登录着」的界面上。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	store := auth.NewSessionStore(redis.Client())
	// 先取用户信息用于日志（会话可能已经没了，取不到就算了）。
	if sess, err := store.Load(ctx, token); err == nil && sess.User != nil {
		// TODO(阶段 3): 登录日志落库，status = Logout。
		log.Printf("[auth] 用户 %s(%d) 退出成功", sess.User.Username, sess.User.UserID)
	}

	if err := store.Delete(ctx, token); err != nil {
		// 会话删不掉是真问题（token 仍然有效），要上报 ——
		// 与上面「日志取不到就算了」不同，这一步失败意味着登出没有生效。
		return err
	}

	// 清在线用户记录，对应 UserActionListener.doLogout。
	if err := redis.Client().Del(ctx, constant.OnlineTokenKeyPrefix+token).Err(); err != nil {
		log.Printf("[auth] 清除在线用户记录失败: %v", err)
	}
	return nil
}
