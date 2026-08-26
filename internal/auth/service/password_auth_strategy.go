package service

import (
	"context"

	"github.com/gin-gonic/gin/binding"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/bcrypt"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// passwordAuthStrategy 密码认证策略（对应 Java PasswordAuthStrategy）。
type passwordAuthStrategy struct{}

// Login 校验账号密码、签发令牌并返回登录结果。body 为原始 JSON 字节，这里解析成 LoginBody。
func (s *passwordAuthStrategy) Login(ctx context.Context, body []byte,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	var loginBody authmodel.PasswordLoginBody
	if err := binding.JSON.BindBody(body, &loginBody); err != nil {
		return nil, errs.New(response.CodeBadRequest, "参数校验失败", err.Error())
	}
	// TODO: 验证码校验属阶段 3，loginBody.Code / loginBody.UUID 字段已就位。
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

	// TODO(阶段 3): 构造登录用户（BuildLoginUser）并签发令牌/写会话/在线记录，待重建。
	return &authvo.LoginVo{ClientID: client.ClientID}, nil
}
