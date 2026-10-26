// Package captcha 短信与邮箱验证码 HTTP 接口。
package captcha

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/mailhelper"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/smshelper"
	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
	rescons "ruoyi-go-vue-plus/internal/resource/constant"
)

// codeExpiration 验证码有效期。
const codeExpiration = rescons.CaptchaExpiration

// mailSubject 验证码邮件主题。
const mailSubject = "登录验证码"

// SmsCode 发送短信验证码。
//
// 出参用 response.Fail 而非 c.Error：返回的是 code=500 的正常响应体，
// 不是抛异常走全局兜底。
func SmsCode(c *gin.Context) {
	phoneNumber := c.Query("phoneNumber")
	if phoneNumber == "" {
		c.JSON(http.StatusOK, response.Fail("用户手机号不能为空"))
		return
	}
	if !rescons.PatternMobile.MatchString(phoneNumber) {
		c.JSON(http.StatusOK, response.Fail("请输入正确的手机号！"))
		return
	}
	if !smshelper.Enabled() {
		c.JSON(http.StatusOK, response.Fail("当前系统没有开启短信功能！"))
		return
	}

	// 签名 / 模板随业务走：验证码这一条用 sms.captcha 子段配的模板，透传给 SendByAli。
	// 模板变量名固定为 "code"，阿里云模板里写 ${code}，故 map 键须与之对应。
	smsTpl := resourceconfig.Get().SMS.Captcha
	code := randomCode()
	if err := smshelper.SendByAli(phoneNumber, smsTpl.SignName, smsTpl.TemplateCode, map[string]string{"code": code}); err != nil {
		// 厂商文案进日志，给前端的是笼统提示：错误里可能带 accessKey 等内部信息。
		log.Printf("[captcha] 验证码短信发送异常 => %v", err)
		c.JSON(http.StatusOK, response.Fail("验证码短信发送失败"))
		return
	}

	// 发送成功才写 Redis：先写后发会让发送失败的码也能通过校验。
	if err := storeCode(c, phoneNumber, code); err != nil {
		log.Printf("[captcha] 缓存短信验证码失败 => %v", err)
		c.JSON(http.StatusOK, response.Fail("验证码短信发送失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// EmailCode 发送邮箱验证码。
func EmailCode(c *gin.Context) {
	ctx := c.Request.Context()

	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusOK, response.Fail("邮箱不能为空"))
		return
	}
	// 开关判断排在格式校验之前：功能没开就没必要挑参数的错。
	if !mailhelper.Enabled() {
		c.JSON(http.StatusOK, response.Fail("当前系统没有开启邮箱功能！"))
		return
	}
	if !rescons.PatternEmail.MatchString(email) {
		c.JSON(http.StatusOK, response.Fail("请输入正确的邮箱地址！"))
		return
	}

	code := randomCode()
	body := fmt.Sprintf("您本次验证码为：%s，有效性为%d分钟，请尽快填写。",
		code, rescons.CaptchaExpirationMinutes)
	if err := mailhelper.Send(ctx, email, mailSubject, body); err != nil {
		// SMTP 报错常含服务器地址与账号，只进日志。
		log.Printf("[captcha] 验证码邮件发送异常 => %v", err)
		c.JSON(http.StatusOK, response.Fail("验证码邮件发送失败"))
		return
	}

	if err := storeCode(c, email, code); err != nil {
		log.Printf("[captcha] 缓存邮箱验证码失败 => %v", err)
		c.JSON(http.StatusOK, response.Fail("验证码邮件发送失败"))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// storeCode 把验证码写进 Redis，键是 手机号/邮箱 本身。
//
// 与图形验证码共用 CaptchaCodeKey 前缀但键不同：图形码的键是随机 uuid，
// 这里直接用手机号/邮箱——登录时前端不回传 uuid，只回传账号与验证码，
// 校验方（internal/auth 的认证策略）只能按账号回查。
func storeCode(c *gin.Context, account, code string) error {
	return redis.Client().
		Set(c.Request.Context(), rescons.CaptchaCodeKey+account, code, codeExpiration).
		Err()
}

// randomCode 生成 4 位数字验证码。
// 前导零要保留，故按字符串格式化而非直接转数字。
func randomCode() string {
	return fmt.Sprintf("%04d", rand.IntN(10000))
}
