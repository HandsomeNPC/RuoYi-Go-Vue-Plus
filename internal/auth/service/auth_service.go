package service

import (
	"net/http"

	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
)

// AuthService 认证业务逻辑。
type AuthService struct{}

// AuthSvcApp 包级实例。
var AuthSvcApp = new(AuthService)

// AuthStrategy 授权策略（对应 Java IAuthStrategy）。
type AuthStrategy interface {
	Login(req *http.Request, body []byte, client *systemvo.SysClientVo) (*authvo.LoginVo, error)
}

// authStrategies 按 grantType 分派授权策略
var authStrategies = map[string]AuthStrategy{
	enum.LoginTypePassword.Code: &passwordAuthStrategy{},
	enum.LoginTypeSms.Code:      &smsAuthStrategy{},
	enum.LoginTypeEmail.Code:    &emailAuthStrategy{},
	enum.LoginTypeSocial.Code:   &socialAuthStrategy{},
}

// Login 登录
func (s *AuthService) Login(req *http.Request, body []byte, grantType string,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	ctx := req.Context()
	strategy, ok := authStrategies[grantType]
	if !ok {
		return nil, errs.New(0, i18n.Msg(ctx, "auth.grant.type.error"), "")
	}
	return strategy.Login(req, body, client)
}
