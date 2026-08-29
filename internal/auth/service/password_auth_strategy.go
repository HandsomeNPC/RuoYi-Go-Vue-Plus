package service

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin/binding"
	goredis "github.com/redis/go-redis/v9"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/bcrypt"
	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/redis"
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
	// 验证码开关，对照 Java `if (captchaEnabled) { validateCaptcha(...) }`。
	if captcha.Enabled() {
		if err := s.validateCaptcha(req, loginBody.Username, loginBody.Code, loginBody.UUID); err != nil {
			return nil, err
		}
	}
	user, err := systemservice.UserSvcApp.LoadUserByUsername(ctx, loginBody.Username)
	if err != nil {
		return nil, err
	}

	if err := SysLoginSvcApp.CheckLogin(req, enum.LoginTypePassword, loginBody.Username,
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

	// 对照 Java UserLoginSuccessListener.handleLoginSuccess 里的 recordLoginInfo。
	// 该监听器还做在线会话/最近登录信息更新，待阶段 3 会话体系落地后再补。
	SysLoginSvcApp.RecordLoginInfo(req, loginBody.Username,
		constant.ConstantLoginSuccess, i18n.Msg(ctx, "user.login.success"))

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    sagin.GetManager().GetConfig().Timeout,
		ClientID:    client.ClientID,
	}, nil
}

// validateCaptcha 校验图形验证码是否有效且匹配，失败时记登录失败日志再返回错误。
// 对照 Java PasswordAuthStrategy.validateCaptcha，四步同序：
// 拼键取值 → 无条件删除 → 判空(失效) → 忽略大小写比对(错误)。
//
// Redis 读写刻意放在本层而非 pkg/captcha：记日志要调 internal/system 的 service，
// 而 pkg 不能 import internal/。Java 各认证策略里这段也是各自一份，未做抽取。
func (s *passwordAuthStrategy) validateCaptcha(req *http.Request, username, code, uuid string) error {
	ctx := req.Context()
	// 对照 Java GlobalConstants.CAPTCHA_CODE_KEY + blankToDefault(uuid, "")。
	verifyKey := constant.CaptchaCodeKey + uuid
	rdb := redis.Client()
	answer, err := rdb.Get(ctx, verifyKey).Result()
	// 无论对错先删，杜绝同一 uuid 反复试错，对照 Java 取值后立即 deleteObject。
	if delErr := rdb.Del(ctx, verifyKey).Err(); delErr != nil {
		log.Printf("[auth] 删除验证码 %s 失败: %v", verifyKey, delErr)
	}

	if err != nil {
		if errors.Is(err, goredis.Nil) {
			// 键不存在(过期或 uuid 无效)，对照 Java CaptchaExpireException。
			msg := i18n.Msg(ctx, "user.jcaptcha.expire")
			SysLoginSvcApp.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
			return errs.New(0, msg, "")
		}
		// Redis 故障不是业务失败，不记登录日志——对照 Java 只在两个 throw 前记。
		return fmt.Errorf("auth: 读取验证码失败: %w", err)
	}
	// 对照 Java StringUtils.equalsIgnoreCase：算术答案大小写无关，字符验证码忽略大小写。
	if !strings.EqualFold(strings.TrimSpace(code), answer) {
		msg := i18n.Msg(ctx, "user.jcaptcha.error")
		SysLoginSvcApp.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
		return errs.New(0, msg, "")
	}
	return nil
}
