package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin/binding"
	goredis "github.com/redis/go-redis/v9"
	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	sysloginservice "ruoyi-go-vue-plus/internal/auth/service/syslogin"
	rescons "ruoyi-go-vue-plus/internal/resource/constant"
	syscons "ruoyi-go-vue-plus/internal/system/constant"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	userservice "ruoyi-go-vue-plus/internal/system/service/user"
)

// smsAuthStrategy 短信验证码认证策略。
type smsAuthStrategy struct{}

// Login 执行短信验证码登录，并按客户端配置生成访问令牌。body 为原始 JSON 字节，解析成 SmsLoginBody。
func (s *smsAuthStrategy) Login(req *http.Request, body []byte,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	ctx := req.Context()
	var loginBody authmodel.SmsLoginBody
	if err := binding.JSON.BindBody(body, &loginBody); err != nil {
		return nil, errs.New(response.CodeBadRequest, "参数校验失败", err.Error())
	}
	phone := loginBody.PhoneNumber
	user, err := userservice.LoadUserByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}

	// 验证码不存在/已过期直接返回（不计重试），匹配与否交由 CheckLogin 闭包判定，
	// 错误验证码才计入重试次数——过期走 error 越过计数、匹配返回 bool 走计数的两态语义。
	matched, err := s.validateSmsCode(req, phone, loginBody.SmsCode)
	if err != nil {
		return nil, err
	}
	if err := sysloginservice.CheckLogin(req, sysloginservice.LoginTypeSms, user.UserName,
		func() bool { return matched }); err != nil {
		return nil, err
	}

	loginUser, err := sysloginservice.BuildLoginUser(req, user)
	if err != nil {
		return nil, err
	}
	loginUser.ClientKey = client.ClientKey
	loginUser.DeviceType = client.DeviceType
	token, err := loginhelper.Login(loginUser, client.DeviceType)
	if err != nil {
		return nil, err
	}

	sysloginservice.RecordOnlineUser(loginUser, token)
	sysloginservice.RecordLoginInfo(req, user.UserName,
		syscons.LoginSuccess, "登录成功")

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    sagin.GetManager().GetConfig().Timeout,
		ClientID:    client.ClientID,
	}, nil
}

// validateSmsCode 校验短信验证码是否存在且匹配。
//
// Go 无异常，故返回 (bool, error)：过期返回 error 由调用方直接返回
// （不进 CheckLogin），匹配返回 bool 供 CheckLogin 闭包作计数判定。
//
// 与图形验证码不同，短信码只读不删：码自有 TTL 自然过期。
func (s *smsAuthStrategy) validateSmsCode(req *http.Request, phone, smsCode string) (bool, error) {
	ctx := req.Context()
	cached, err := redis.Client().Get(ctx, rescons.CaptchaCodeKey+phone).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		// Redis 故障不是业务失败，不记登录日志。
		return false, errs.New(0, "短信验证码读取失败", err.Error())
	}
	if cached == "" {
		// 验证码为空即过期。
		msg := "验证码已失效"
		sysloginservice.RecordLoginInfo(req, phone, syscons.LoginFail, msg)
		return false, errs.New(0, msg, "")
	}
	return cached == smsCode, nil
}
