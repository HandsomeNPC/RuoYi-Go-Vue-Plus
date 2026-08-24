// Package service auth 模块业务逻辑层。
//
// in-process 复用 internal/system 的 service(用户/角色/菜单)，直接函数调用，
// 无网络开销。因此 auth 进程需连接同一数据库。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
)

// AuthService 认证业务逻辑：登录、登出。
//
// # 架构验证点（M1）
//
// 它直接持有 system 模块的 UserService / ClientService（同进程函数调用），
// **没有任何 HTTP 客户端** —— 这就是 CLAUDE.md 里「auth 复用 system 走
// in-process」那条约定的落地形态。auth 进程因此也要连同一个数据库。
type AuthService struct {
	users   *systemservice.UserService
	clients *systemservice.ClientService

	sessions *auth.SessionStore
	rdb      goredis.UniversalClient

	jwtSecret string
	passwdCfg config.UserPassword

	// strategies 按 grantType 分派登录策略。
	//
	// 对应 Java 的 IAuthStrategy.login：那边靠拼 Spring bean 名
	// （grantType + "AuthStrategy"）从容器里取，Go 没有容器，用一张表。
	// 本阶段只注册 password；短信/邮箱/社交/小程序留阶段 4
	// （届时各自实现 loginStrategy 并在 NewAuthService 里注册一行）。
	strategies map[string]loginStrategy
}

// loginStrategy 一种登录方式的凭证校验，对应 Java 的 IAuthStrategy。
//
// 返回校验通过的用户。**只负责「证明你是你」**：查客户端、签 token、
// 写会话那些所有策略共通的步骤在 Login 里，不重复实现 ——
// Java 侧每个策略都自己调一遍 buildLoginUser/LoginHelper.login，
// 那是重复代码，新增策略时容易漏掉某一步。
type loginStrategy func(ctx context.Context, body *authmodel.LoginBody,
	client *systemmodel.SysClient) (*systemmodel.SysUser, error)

// NewAuthService 构造认证 service。
//
// db 与 rdb 由 cmd/auth 注入 —— 本层不调 database.DB() / redis.Client()，
// 那两个包级取值器会 panic，不该出现在可被测试的业务代码里。
func NewAuthService(db *gorm.DB, rdb goredis.UniversalClient, cfg *config.Config) *AuthService {
	s := &AuthService{
		users:     systemservice.NewUserService(db),
		clients:   systemservice.NewClientService(db),
		sessions:  auth.NewSessionStore(rdb),
		rdb:       rdb,
		jwtSecret: cfg.JWT.Secret,
		passwdCfg: cfg.User.Password,
	}
	s.strategies = map[string]loginStrategy{
		enum.LoginTypePassword.Code: s.passwordLogin,
	}
	return s
}

// Login 登录，对应原项目 AuthController.login + IAuthStrategy.login 那条链路。
//
// # 步骤顺序有安全含义，不要调整
//
//  1. 查客户端，校验授权类型与状态
//  2. 按 grantType 分派策略，校验凭证（密码策略见 passwordLogin）
//  3. 构造 LoginUser
//  4. 签 JWT + 写 Redis 会话
//  5. 副作用：在线用户记录、更新最后登录信息
//
// 第 2 步内部的顺序更要紧（「用户不存在」必须早于「密码错误计数」），
// 详见 passwordLogin 的说明。
func (s *AuthService) Login(ctx context.Context, body *authmodel.LoginBody, clientIP string) (*authmodel.LoginVo, error) {
	// 1. 查客户端并校验。
	client, err := s.clients.GetByClientID(ctx, body.ClientID)
	if err != nil {
		if errors.Is(err, systemservice.ErrClientNotFound) {
			// 与「授权类型不支持」折叠成同一句，对齐 Java：
			// AuthController.java:85-88 对 client 为 null 和 grantType 不含
			// 两种情况回的都是 auth.grant.type.error。
			// 不区分也更好 —— 否则这个接口就成了枚举有效 clientId 的工具。
			log.Printf("[auth] 客户端id: %s 认证类型: %s 异常", body.ClientID, body.GrantType)
			return nil, errs.New(i18n.Msg(ctx, "auth.grant.type.error"))
		}
		return nil, err
	}
	if !client.SupportsGrantType(body.GrantType) {
		log.Printf("[auth] 客户端id: %s 不支持认证类型: %s", body.ClientID, body.GrantType)
		return nil, errs.New(i18n.Msg(ctx, "auth.grant.type.error"))
	}
	if client.Status != constant.StatusNormal {
		return nil, errs.New(i18n.Msg(ctx, "auth.grant.type.blocked"))
	}

	// 2. 分派策略校验凭证。
	strategy, ok := s.strategies[body.GrantType]
	if !ok {
		// 走到这里说明 sys_client.grant_type 里配了一个还没实现的类型
		// （如阶段 4 才做的 sms）。对齐 Java 的 ServiceException("授权类型不正确!")。
		return nil, errs.New(i18n.Msg(ctx, "auth.grant.type.error"))
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

// passwordLogin 密码登录，对应原项目 PasswordAuthStrategy。
//
// # 三步的顺序是安全要求
//
//  1. 查用户：不存在 / 已停用 -> 直接失败
//  2. 错误次数前置检查：已达上限 -> 直接失败（不比密码）
//  3. BCrypt 比对：失败则递增计数，成功则清零
//
// **第 1 步必须早于第 2、3 步**（对齐 Java 的 loadUserByUsername 先行）：
// 用户不存在时直接抛，压根不碰错误计数器。否则攻击者能用任意不存在的
// 用户名把某个真实账号刷到锁定 —— 计数键是按 username 存的，
// 而「这个用户名存不存在」在锁定之前是未知的。
//
// 代价是**用户枚举可观测**：不存在回「账号不存在」、密码错回「密码输入错误N次」，
// 两句文案不同。这是原项目的行为，本次对齐 —— 改掉它需要连同前端的
// 提示逻辑一起改，且那属于安全加固而非迁移。
func (s *AuthService) passwordLogin(ctx context.Context, body *authmodel.LoginBody,
	_ *systemmodel.SysClient) (*systemmodel.SysUser, error) {

	// TODO(阶段 3): 验证码校验。原项目 captcha.enable 默认 false
	// （application.yml:20），且验证码生成接口属阶段 3 —— 现在校验会让
	// 谁都登不进来。body.Code / body.UUID 字段已就位。

	// 1. 查用户并校验状态。
	user, err := s.users.GetByUsername(ctx, body.Username)
	if err != nil {
		if errors.Is(err, systemservice.ErrUserNotFound) {
			log.Printf("[auth] 登录用户: %s 不存在", body.Username)
			return nil, errs.New(i18n.Msg(ctx, "user.not.exists", body.Username))
		}
		return nil, err
	}
	if user.Status == enum.UserStatusDisable.Code {
		log.Printf("[auth] 登录用户: %s 已被停用", body.Username)
		return nil, errs.New(i18n.Msg(ctx, "user.blocked", body.Username))
	}

	// 2 + 3. 错误次数检查与密码比对。
	if err := s.checkPassword(ctx, body.Username, body.Password, user.Password); err != nil {
		return nil, err
	}
	return user, nil
}

// checkPassword 校验密码并维护错误次数，对应 Java SysLoginService.checkLogin（:206-235）。
//
// Redis 键 pwd_err_cnt:<username>，值是计数，TTL = lockTime。
//
// **TTL 每次失败都重置**（滑动窗口，非固定窗口）—— 这是原项目的行为：
// 每 9 分钟错一次、错满 5 次跨越 40 分钟，依然会锁。
func (s *AuthService) checkPassword(ctx context.Context, username, password, hashed string) error {
	key := constant.PwdErrCntKeyPrefix + username
	maxRetry := s.passwdCfg.MaxRetryCount
	lockMinutes := s.passwdCfg.LockTime

	// 前置检查：已达上限则直接拒绝，不再比对密码。
	errCount, err := s.rdb.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		// Redis 故障不该让登录直接不可用，但也不能静默跳过限制 ——
		// 那等于在 Redis 挂掉时打开暴力破解的门。折中：当作 0 次继续，
		// 并打 error 级日志。密码本身仍然要比对，所以不是「放行」。
		log.Printf("[auth] 读取密码错误次数失败(按 0 次继续): %v", err)
		errCount = 0
	}
	if errCount >= maxRetry {
		return errs.New(i18n.Msg(ctx, enum.LoginTypePassword.RetryExceedKey, maxRetry, lockMinutes))
	}

	// 密码比对。
	verifyErr := auth.VerifyPassword(password, hashed)
	if verifyErr == nil {
		// 成功：清零计数。
		if err := s.rdb.Del(ctx, key).Err(); err != nil {
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
	if err := s.rdb.Set(ctx, key, errCount,
		time.Duration(lockMinutes)*time.Minute).Err(); err != nil {
		log.Printf("[auth] 写入密码错误次数失败: %v", err)
	}

	if errCount >= maxRetry {
		return errs.New(i18n.Msg(ctx, enum.LoginTypePassword.RetryExceedKey, maxRetry, lockMinutes))
	}
	return errs.New(i18n.Msg(ctx, enum.LoginTypePassword.RetryCountKey, errCount))
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

	token, err := auth.Sign(claims, s.jwtSecret, ttl)
	if err != nil {
		return nil, err
	}

	loginUser.Token = token
	loginUser.ExpireTime = time.Now().Add(ttl).UnixMilli()

	sess := &auth.Session{User: loginUser, ActiveTimeout: client.ActiveTimeout}
	if err := s.sessions.Save(ctx, token, sess); err != nil {
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
	if err := s.users.UpdateLoginInfo(ctx, loginUser.UserID, loginUser.IPAddr); err != nil {
		log.Printf("[auth] 更新用户 %d 最后登录信息失败: %v", loginUser.UserID, err)
	}

	// TODO(阶段 3): 登录日志落库（sys_login_info 表 + LoginInfoEvent 等价物）。
	// 现在只打日志 —— 表结构与 repository 属阶段 3。
	log.Printf("[auth] 用户 %s(%d) 登录成功, ip=%s, client=%s",
		loginUser.Username, loginUser.UserID, loginUser.IPAddr, client.ClientKey)
}

// onlineUser 在线用户记录，对应 Java 的 UserOnlineDTO。
type onlineUser struct {
	TokenID       string `json:"tokenId"`
	UserName      string `json:"userName"`
	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`
	DeptName      string `json:"deptName"`
	ClientKey     string `json:"clientKey"`
	DeviceType    string `json:"deviceType"`
	LoginTime     int64  `json:"loginTime"`
}

// recordOnline 写在线用户记录。
//
// TTL 取 client.Timeout（绝对超时），对齐 Java：
// timeout == -1 时不设过期，否则 Duration.ofSeconds(timeout)。
func (s *AuthService) recordOnline(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient, token string) {

	dto := onlineUser{
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
	if err := s.rdb.Set(ctx, constant.OnlineTokenKeyPrefix+token, payload, ttl).Err(); err != nil {
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

	// 先取用户信息用于日志（会话可能已经没了，取不到就算了）。
	if sess, err := s.sessions.Load(ctx, token); err == nil && sess.User != nil {
		// TODO(阶段 3): 登录日志落库，status = Logout。
		log.Printf("[auth] 用户 %s(%d) 退出成功", sess.User.Username, sess.User.UserID)
	}

	if err := s.sessions.Delete(ctx, token); err != nil {
		// 会话删不掉是真问题（token 仍然有效），要上报 ——
		// 与上面「日志取不到就算了」不同，这一步失败意味着登出没有生效。
		return err
	}

	// 清在线用户记录，对应 UserActionListener.doLogout。
	if err := s.rdb.Del(ctx, constant.OnlineTokenKeyPrefix+token).Err(); err != nil {
		log.Printf("[auth] 清除在线用户记录失败: %v", err)
	}
	return nil
}
