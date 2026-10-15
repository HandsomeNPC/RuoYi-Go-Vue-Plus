package handler

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/captchahelper"
	"github.com/HandsomeNPC/ruoyi-common-go/oauth2"
	"github.com/HandsomeNPC/ruoyi-common-go/redis"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
	"ruoyi-go-vue-plus/internal/auth/model"
	"ruoyi-go-vue-plus/internal/auth/model/vo"
	authservice "ruoyi-go-vue-plus/internal/auth/service/auth"
	socialauth "ruoyi-go-vue-plus/internal/auth/service/social"
	sysloginservice "ruoyi-go-vue-plus/internal/auth/service/syslogin"
	rescons "ruoyi-go-vue-plus/internal/resource/constant"
	syscons "ruoyi-go-vue-plus/internal/system/constant"
	clientservice "ruoyi-go-vue-plus/internal/system/service/client"
	socialservice "ruoyi-go-vue-plus/internal/system/service/social"
)

// codeExpiration 图形验证码有效期。裸 int(分钟) 在 constant 包，此处补单位。
const codeExpiration = rescons.CaptchaExpiration

// AuthApi 认证接口。
type AuthApi struct{}

var AuthApiApp = new(AuthApi)

// Login 登录。
func (a *AuthApi) Login(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "读取请求体失败", err.Error()))
		return
	}
	var body model.LoginBody
	if err := binding.JSON.BindBody(raw, &body); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	clientID := body.ClientID
	grantType := body.GrantType
	client, err := clientservice.QueryByClientID(c.Request.Context(), clientID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 查询不到 client 或 client 内不包含 grantType
	if grantType == "" || !slices.Contains(client.GrantTypeList, grantType) {
		log.Printf("[auth] 客户端id: %s 认证类型: %s 异常", clientID, grantType)
		_ = c.Error(errs.New(0, "认证权限类型错误", ""))
		return
	} else if client.Status != syscons.StatusNormal {
		// 客户端已停用
		_ = c.Error(errs.New(0, "认证权限类型已禁用", ""))
		return
	}
	vo, err := authservice.Login(c.Request, raw, grantType, client)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}

// Logout 登出。
// 恒返回成功：service 内部吞掉未登录/token 失效的情况，前端总能正常清态。
func (a *AuthApi) Logout(c *gin.Context) {
	// token 由组级 TokenInterceptor 解析后存入 ctx；service 层不碰 *gin.Context，故在此取出传入。
	sysloginservice.Logout(c.Request, sagin.GetTokenFromCtx(c))

	c.JSON(http.StatusOK, response.OkMsg("退出成功"))
}

// Code 获取图形验证码。
//
// 是否启用由 captchahelper.Enabled() 判定：关闭时直接回 captchaEnabled=false 的
// 空响应，前端据此隐藏验证码输入。启用时 captchahelper.Generate 出题画图，
// 答案写 Redis（键 = CaptchaCodeKey + uuid，TTL 取 ConstantCaptchaExpiration 分钟），
// 校验(取值→删除→判空→比对)在 password_auth_strategy.validateCaptcha。
func (a *AuthApi) Code(c *gin.Context) {
	if !captchahelper.Enabled() {
		c.JSON(http.StatusOK, response.Ok(vo.CaptchaVo{CaptchaEnabled: false}))
		return
	}
	uuid, img, answer, err := captchahelper.Generate()
	if err != nil {
		_ = c.Error(err)
		return
	}
	// Generate 回带 "data:image/png;base64," 前缀，前端自己拼前缀，此处剥成裸 base64。
	img = strings.TrimPrefix(img, "data:image/png;base64,")
	key := rescons.CaptchaCodeKey + uuid
	if err := redis.Client().Set(c.Request.Context(), key, answer, codeExpiration).Err(); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(vo.CaptchaVo{CaptchaEnabled: true, UUID: uuid, Img: img}))
}

// Binding 获取三方授权跳转地址。
//
// 公开接口：登录页未登录时点三方登录也要拿得到地址。
func (a *AuthApi) Binding(c *gin.Context) {
	source := c.Param("source")
	url, err := socialauth.AuthorizeURL(c.Request.Context(), source)
	if err != nil {
		if errors.Is(err, oauth2.ErrUnsupportedSource) {
			_ = c.Error(errs.New(0, source+"平台账号暂不支持", ""))
			return
		}
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(url))
}

// SocialCallback 三方回调后绑定账号。
func (a *AuthApi) SocialCallback(c *gin.Context) {
	var body model.SocialLoginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	ctx := c.Request.Context()
	authUser, err := socialauth.Callback(ctx, body.Source, body.SocialCode, body.SocialState)
	if err != nil {
		if errors.Is(err, oauth2.ErrUnsupportedSource) {
			_ = c.Error(errs.New(0, body.Source+"平台账号暂不支持", ""))
			return
		}
		if errors.Is(err, oauth2.ErrIllegalState) {
			_ = c.Error(errs.New(0, "三方登录状态已失效，请重新授权", err.Error()))
			return
		}
		_ = c.Error(err)
		return
	}

	if err := sysloginservice.SocialRegister(ctx,
		loginhelper.GetUserID(c), authUser); err != nil {
		if errors.Is(err, socialservice.ErrSocialAlreadyBound) {
			_ = c.Error(errs.New(0, "此三方账号已经被绑定!", ""))
			return
		}
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.OkVoid())
}

// UnlockSocial 取消三方账号授权。
func (a *AuthApi) UnlockSocial(c *gin.Context) {
	raw := c.Param("socialId")
	// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
	socialID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || socialID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", raw))
		return
	}

	ctx := c.Request.Context()
	// 校验归属：原实现只按主键删，任何登录用户都能解绑他人账号，而那行里存着三方 access_token。
	owner, err := socialservice.FindOwnerUserID(ctx, socialID)
	if err != nil {
		if errors.Is(err, socialservice.ErrSocialNotFound) {
			_ = c.Error(errs.New(0, "取消授权失败", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	if owner != loginhelper.GetUserID(c) {
		_ = c.Error(errs.New(0, "取消授权失败", "社会化绑定不属于当前用户"))
		return
	}

	ok, err := socialservice.DeleteWithValidById(ctx, socialID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if !ok {
		_ = c.Error(errs.New(0, "取消授权失败", ""))
		return
	}

	c.JSON(http.StatusOK, response.OkVoid())
}
