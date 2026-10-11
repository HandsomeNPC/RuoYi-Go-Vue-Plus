package auth

import (
	"net/http"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/enum"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
)

// AuthStrategy 授权策略。
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
func Login(req *http.Request, body []byte, grantType string,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	strategy, ok := authStrategies[grantType]
	if !ok {
		return nil, errs.New(0, "认证权限类型错误", "")
	}
	return strategy.Login(req, body, client)
}
