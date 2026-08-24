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

// AuthService 认证业务逻辑。
type AuthService struct{}

// AuthSvcApp 包级实例。
var AuthSvcApp = new(AuthService)

// loginStrategy 一种登录方式的凭证校验。
type loginStrategy func(ctx context.Context, body *authmodel.LoginBody,
	client *systemmodel.SysClient) (*systemmodel.SysUser, error)

// strategies 按 grantType 分派登录策略。
var strategies = map[string]loginStrategy{
	enum.LoginTypePassword.Code: AuthSvcApp.passwordLogin,
}

// Login 登录。
func (s *AuthService) Login(ctx context.Context, body *authmodel.LoginBody, clientIP string) (*authmodel.LoginVo, error) {
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

	strategy, ok := strategies[body.GrantType]
	if !ok {
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
	}
	user, err := strategy(ctx, body, client)
	if err != nil {
		return nil, err
	}

	loginUser := s.buildLoginUser(user, client, clientIP)

	vo, err := s.issue(ctx, loginUser, client)
	if err != nil {
		return nil, err
	}

	s.afterLogin(ctx, loginUser, client, vo.AccessToken)

	return vo, nil
}

// passwordLogin 密码登录。
func (s *AuthService) passwordLogin(ctx context.Context, body *authmodel.LoginBody,
	_ *systemmodel.SysClient) (*systemmodel.SysUser, error) {

	// TODO: 验证码校验属阶段 3，body.Code / body.UUID 字段已就位。

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

	if err := s.checkPassword(ctx, body.Username, body.Password, user.Password); err != nil {
		return nil, err
	}
	return user, nil
}

// checkPassword 校验密码并维护错误次数。
func (s *AuthService) checkPassword(ctx context.Context, username, password, hashed string) error {
	key := constant.PwdErrCntKeyPrefix + username
	passwdCfg := config.Get().User.Password
	maxRetry := passwdCfg.MaxRetryCount
	lockMinutes := passwdCfg.LockTime

	rdb := redis.Client()
	errCount, err := rdb.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		log.Printf("[auth] 读取密码错误次数失败(按 0 次继续): %v", err)
		errCount = 0
	}
	if errCount >= maxRetry {
		return errs.New(0, i18n.Msg(ctx, enum.LoginTypePassword.RetryExceedKey, maxRetry, lockMinutes), "")
	}

	verifyErr := auth.VerifyPassword(password, hashed)
	if verifyErr == nil {
		if err := rdb.Del(ctx, key).Err(); err != nil {
			log.Printf("[auth] 清除密码错误次数失败: %v", err)
		}
		return nil
	}
	if !errors.Is(verifyErr, auth.ErrPasswordMismatch) {
		log.Printf("[auth] 用户 %s 的密码哈希格式非法: %v", username, verifyErr)
		return fmt.Errorf("密码校验失败: %w", verifyErr)
	}

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

// buildLoginUser 构造登录用户。
func (s *AuthService) buildLoginUser(user *systemmodel.SysUser,
	client *systemmodel.SysClient, clientIP string) *auth.LoginUser {

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
		IPAddr:    clientIP,
		// TODO(阶段 3): LoginLocation 需 IP 归属地库，Browser / OS 需解析 User-Agent。

		ClientKey:  client.ClientKey,
		DeviceType: client.DeviceType,
	}
}

// issue 签发 token 并写入会话。
func (s *AuthService) issue(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient) (*authmodel.LoginVo, error) {

	loginID, ok := loginUser.LoginID()
	if !ok {
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

// afterLogin 登录成功的副作用，失败只打日志不影响登录结果。
func (s *AuthService) afterLogin(ctx context.Context, loginUser *auth.LoginUser,
	client *systemmodel.SysClient, token string) {

	s.recordOnline(ctx, loginUser, client, token)

	if err := systemservice.UserSvcApp.UpdateLoginInfo(ctx, loginUser.UserID, loginUser.IPAddr); err != nil {
		log.Printf("[auth] 更新用户 %d 最后登录信息失败: %v", loginUser.UserID, err)
	}

	// TODO(阶段 3): 登录日志落库。
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

	var ttl time.Duration
	if client.Timeout > 0 {
		ttl = time.Duration(client.Timeout) * time.Second
	}
	if err := redis.Client().Set(ctx, constant.OnlineTokenKeyPrefix+token, payload, ttl).Err(); err != nil {
		log.Printf("[auth] 写入在线用户记录失败: %v", err)
	}
}

// Logout 登出，幂等。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	store := auth.NewSessionStore(redis.Client())
	if sess, err := store.Load(ctx, token); err == nil && sess.User != nil {
		// TODO(阶段 3): 登录日志落库，status = Logout。
		log.Printf("[auth] 用户 %s(%d) 退出成功", sess.User.Username, sess.User.UserID)
	}

	if err := store.Delete(ctx, token); err != nil {
		return err
	}

	if err := redis.Client().Del(ctx, constant.OnlineTokenKeyPrefix+token).Err(); err != nil {
		log.Printf("[auth] 清除在线用户记录失败: %v", err)
	}
	return nil
}
