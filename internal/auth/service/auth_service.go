package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/redis"
)

// AuthService 认证业务逻辑。
type AuthService struct{}

// AuthSvcApp 包级实例。
var AuthSvcApp = new(AuthService)

// AuthStrategy 授权策略（对应 Java IAuthStrategy）。
type AuthStrategy interface {
	Login(ctx context.Context, body []byte, client *systemvo.SysClientVo) (*systemmodel.SysUser, error)
}

// authStrategies 按 grantType 分派授权策略
var authStrategies = map[string]AuthStrategy{
	enum.LoginTypePassword.Code: &passwordAuthStrategy{},
}

// Login 登录
func (s *AuthService) Login(req *http.Request, body []byte, grantType string,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {

	ctx := req.Context()

	strategy, ok := authStrategies[grantType]
	if !ok {
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
	}
	user, err := strategy.Login(ctx, body, client)
	if err != nil {
		return nil, err
	}
	loginUser := SysLoginSvcApp.BuildLoginUser(user, client)
	vo, err := s.issue(ctx, loginUser, client)
	if err != nil {
		return nil, err
	}
	s.afterLogin(req, loginUser, client, vo.AccessToken)
	return vo, nil
}

// BuildLoginUser 见 sys_login_service.go 的 SysLoginService.BuildLoginUser。

// issue 签发 token 并写入会话。
func (s *AuthService) issue(ctx context.Context, loginUser *auth.LoginUser,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {

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

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    client.Timeout,
		ClientID:    client.ClientID,
	}, nil
}

// afterLogin 登录成功的副作用，失败只打日志不影响登录结果。
func (s *AuthService) afterLogin(req *http.Request, loginUser *auth.LoginUser,
	client *systemvo.SysClientVo, token string) {

	ctx := req.Context()
	// IP 从请求直接取（对应 Java 在 record 阶段 ServletUtils.getClientIP）。
	loginUser.IPAddr = ip.ClientIP(req)

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
	client *systemvo.SysClientVo, token string) {

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
