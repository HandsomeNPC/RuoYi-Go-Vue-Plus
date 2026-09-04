package service

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin/binding"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/social"
)

// socialAuthStrategy 第三方授权认证策略。
type socialAuthStrategy struct{}

// Login 用三方授权码换用户资料、按绑定关系回查系统用户并签发令牌。
// body 为原始 JSON 字节，解析成 SocialLoginBody（source/socialCode/socialState）。
//
// 与密码/短信/邮箱策略不同：三方登录不做重试计数（LoginTypeSocial 词条键为空），
// 也不校验验证码——授权有效性由 state 单次有效与 code 换令牌两步兜住。
// 错误映射对齐 auth_handler 的绑定回调：不支持的平台与失效的 state 各自一条文案。
func (s *socialAuthStrategy) Login(req *http.Request, body []byte,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	ctx := req.Context()
	var loginBody authmodel.SocialLoginBody
	if err := binding.JSON.BindBody(body, &loginBody); err != nil {
		return nil, errs.New(response.CodeBadRequest, "参数校验失败", err.Error())
	}

	authUser, err := social.LoginAuth(ctx, loginBody.Source, loginBody.SocialCode, loginBody.SocialState)
	if err != nil {
		if errors.Is(err, social.ErrUnsupportedSource) {
			return nil, errs.New(0, loginBody.Source+"平台账号暂不支持", "")
		}
		if errors.Is(err, social.ErrIllegalState) {
			return nil, errs.New(0, "三方登录状态已失效，请重新授权", err.Error())
		}
		return nil, err
	}

	// authId = source + 平台内唯一标识，是定位绑定关系的全局键。
	list, err := systemservice.SocialSvcApp.SelectByAuthId(ctx, authUser.Source+authUser.UUID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errs.New(0, "你还没有绑定第三方账号，绑定后才可以登录！", "")
	}

	user, err := systemservice.UserSvcApp.LoadUserByID(ctx, list[0].UserID)
	if err != nil {
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

	SysLoginSvcApp.RecordOnlineUser(loginUser, token)
	SysLoginSvcApp.RecordLoginInfo(req, user.UserName,
		constant.ConstantLoginSuccess, i18n.Msg(ctx, "user.login.success"))

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    sagin.GetManager().GetConfig().Timeout,
		ClientID:    client.ClientID,
	}, nil
}
