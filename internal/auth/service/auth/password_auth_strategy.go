package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/HandsomeNPC/ruoyi-common-go/bcrypt"
	"github.com/gin-gonic/gin/binding"
	goredis "github.com/redis/go-redis/v9"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/captcha"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/constant"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/enum"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken/loginhelper"
	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	authvo "ruoyi-go-vue-plus/internal/auth/model/vo"
	sysloginservice "ruoyi-go-vue-plus/internal/auth/service/syslogin"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	userservice "ruoyi-go-vue-plus/internal/system/service/user"
)

// passwordAuthStrategy 密码认证策略。
type passwordAuthStrategy struct{}

// Login 校验账号密码、签发令牌并返回登录结果。body 为原始 JSON 字节，解析成 PasswordLoginBody。
func (s *passwordAuthStrategy) Login(req *http.Request, body []byte,
	client *systemvo.SysClientVo) (*authvo.LoginVo, error) {
	ctx := req.Context()
	var loginBody authmodel.PasswordLoginBody
	if err := binding.JSON.BindBody(body, &loginBody); err != nil {
		return nil, errs.New(response.CodeBadRequest, "参数校验失败", err.Error())
	}
	// 验证码开关。
	if captcha.Enabled() {
		if err := s.validateCaptcha(req, loginBody.Username, loginBody.Code, loginBody.UUID); err != nil {
			return nil, err
		}
	}
	user, err := userservice.LoadUserByUsername(ctx, loginBody.Username)
	if err != nil {
		return nil, err
	}

	if err := sysloginservice.CheckLogin(req, enum.LoginTypePassword, loginBody.Username,
		func() bool {
			return bcrypt.Checkpw(loginBody.Password, user.Password)
		}); err != nil {
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

	// 先写在线摘要再记登录日志。
	// 在线摘要须在 loginhelper.Login 返回后写（见 RecordOnlineUser 注释）。
	sysloginservice.RecordOnlineUser(loginUser, token)
	sysloginservice.RecordLoginInfo(req, loginBody.Username,
		constant.ConstantLoginSuccess, "登录成功")

	return &authvo.LoginVo{
		AccessToken: token,
		ExpireIn:    sagin.GetManager().GetConfig().Timeout,
		ClientID:    client.ClientID,
	}, nil
}

// validateCaptcha 校验图形验证码是否有效且匹配，失败时记登录失败日志再返回错误。
//
// Redis 读写刻意放在本层而非 pkg/captcha：记日志要调 internal/system 的 service，
// 而 pkg 不能 import internal/。
func (s *passwordAuthStrategy) validateCaptcha(req *http.Request, username, code, uuid string) error {
	ctx := req.Context()
	verifyKey := constant.CaptchaCodeKey + uuid
	rdb := redis.Client()
	answer, err := rdb.Get(ctx, verifyKey).Result()
	// 无论对错先删，杜绝同一 uuid 反复试错。
	if delErr := rdb.Del(ctx, verifyKey).Err(); delErr != nil {
		log.Printf("[auth] 删除验证码 %s 失败: %v", verifyKey, delErr)
	}

	if err != nil {
		if errors.Is(err, goredis.Nil) {
			// 键不存在(过期或 uuid 无效)。
			msg := "验证码已失效"
			sysloginservice.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
			return errs.New(0, msg, "")
		}
		// Redis 故障不是业务失败，不记登录日志。
		return fmt.Errorf("auth: 读取验证码失败: %w", err)
	}
	// 算术答案大小写无关，字符验证码忽略大小写。
	if !strings.EqualFold(strings.TrimSpace(code), answer) {
		msg := "验证码错误"
		sysloginservice.RecordLoginInfo(req, username, constant.ConstantLoginFail, msg)
		return errs.New(0, msg, "")
	}
	return nil
}
