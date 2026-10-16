package auth

import (
	"net/http"

	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	sysloginservice "ruoyi-go-vue-plus/internal/auth/service/syslogin"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	authmodel "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
)

// AuthStrategy 授权策略。
type AuthStrategy interface {
	Login(req *http.Request, body []byte, client *systemvo.SysClientVo) (*authvo.LoginVo, error)
}

// authStrategies 按 grantType 分派授权策略
var authStrategies = map[string]AuthStrategy{
	sysloginservice.LoginTypePassword.Code: &passwordAuthStrategy{},
	sysloginservice.LoginTypeSms.Code:      &smsAuthStrategy{},
	sysloginservice.LoginTypeEmail.Code:    &emailAuthStrategy{},
	sysloginservice.LoginTypeSocial.Code:   &socialAuthStrategy{},
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

// loginByClient 按 sys_client 配的超时签发令牌并组装登录结果，供各策略复用。
//
// 超时按客户端存库（pc / app / 小程序各一行），故须随登录传入；
// 只读 satoken 全局配置会让所有客户端共用一套有效期。
func loginByClient(loginUser *authmodel.LoginUser,
	client *systemvo.SysClientVo) (string, *authvo.LoginVo, error) {
	loginUser.ClientKey = client.ClientKey
	loginUser.DeviceType = client.DeviceType

	token, err := loginhelper.LoginWithOptions(loginUser, loginhelper.LoginOptions{
		Device:        client.DeviceType,
		Timeout:       client.Timeout,
		ActiveTimeout: client.ActiveTimeout,
	})
	if err != nil {
		return "", nil, err
	}

	return token, &authvo.LoginVo{
		AccessToken: token,
		// 回传本次登录的真实剩余有效期，而非全局配置值
		ExpireIn: tokenExpireIn(token, client),
		ClientID: client.ClientID,
	}, nil
}

// tokenExpireIn 取 token 实际剩余有效期(秒)。
// 读不到或被配成永不过期时回落 client 配的值，避免给前端返回 -1/-2。
func tokenExpireIn(token string, client *systemvo.SysClientVo) int64 {
	if ttl, err := sagin.GetManager().GetTokenTimeout(token); err == nil && ttl > 0 {
		return ttl
	}
	return client.Timeout
}
