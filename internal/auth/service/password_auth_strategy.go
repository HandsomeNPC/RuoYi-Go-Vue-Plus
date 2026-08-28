package service

import (
	"net/http"

	"github.com/gin-gonic/gin/binding"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/bcrypt"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// passwordAuthStrategy 密码认证策略（对应 Java PasswordAuthStrategy）。
type passwordAuthStrategy struct{}

// Login 校验账号密码、签发令牌并返回登录结果。body 为原始 JSON 字节，解析成 PasswordLoginBody。
func (s *passwordAuthStrategy) Login(req *http.Request, body []byte,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	ctx := req.Context()
	var loginBody authmodel.PasswordLoginBody
	if err := binding.JSON.BindBody(body, &loginBody); err != nil {
		return nil, errs.New(response.CodeBadRequest, "参数校验失败", err.Error())
	}
	// TODO: 验证码校验
	user, err := systemservice.UserSvcApp.LoadUserByUsername(ctx, loginBody.Username)
	if err != nil {
		return nil, err
	}

	if err := SysLoginSvcApp.CheckLogin(ctx, enum.LoginTypePassword, loginBody.Username,
		func() bool {
			return bcrypt.Checkpw(loginBody.Password, user.Password)
		}); err != nil {
		return nil, err
	}

	loginUser, err := SysLoginSvcApp.BuildLoginUser(req, user)
	if err != nil {
		return nil, err
	}
	loginUser.ClientKey = client.ClientKey
	loginUser.DeviceType = client.DeviceType
	token, err := loginhelper.Login(loginUser, client.DeviceType)
	if err != nil {
		return nil, err
	}

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    sagin.GetManager().GetConfig().Timeout,
		ClientID:    client.ClientID,
	}, nil
}
