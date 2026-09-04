package handler

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/auth/model"
	authservice "ruoyi-go-vue-plus/internal/auth/service"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/i18n"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/social"
)

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
	client, err := systemservice.ClientSvcApp.QueryByClientID(c.Request.Context(), clientID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 查询不到 client 或 client 内不包含 grantType
	if grantType == "" || !slices.Contains(client.GrantTypeList, grantType) {
		log.Printf("[auth] 客户端id: %s 认证类型: %s 异常", clientID, grantType)
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), "auth.grant.type.error"), ""))
		return
	} else if client.Status != constant.StatusNormal {
		// 客户端已停用
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), "auth.grant.type.blocked"), ""))
		return
	}
	vo, err := authservice.AuthSvcApp.Login(c.Request, raw, grantType, client)
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
	authservice.SysLoginSvcApp.Logout(c.Request, sagin.GetTokenFromCtx(c))

	c.JSON(http.StatusOK, response.OkMsg(i18n.Msg(c.Request.Context(), "user.logout.success")))
}

// Code 获取图形验证码。
func (a *AuthApi) Code(c *gin.Context) {
	vo, err := captcha.Generate(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}

// Binding 获取三方授权跳转地址。
//
// 公开接口：登录页未登录时点三方登录也要拿得到地址。
func (a *AuthApi) Binding(c *gin.Context) {
	source := c.Param("source")
	url, err := social.GetAuthorizeURL(c.Request.Context(), source)
	if err != nil {
		if errors.Is(err, social.ErrUnsupportedSource) {
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
	authUser, err := social.LoginAuth(ctx, body.Source, body.SocialCode, body.SocialState)
	if err != nil {
		if errors.Is(err, social.ErrUnsupportedSource) {
			_ = c.Error(errs.New(0, body.Source+"平台账号暂不支持", ""))
			return
		}
		if errors.Is(err, social.ErrIllegalState) {
			_ = c.Error(errs.New(0, "三方登录状态已失效，请重新授权", err.Error()))
			return
		}
		_ = c.Error(err)
		return
	}

	if err := authservice.SysLoginSvcApp.SocialRegister(ctx,
		loginhelper.GetUserID(c), authUser); err != nil {
		if errors.Is(err, systemservice.ErrSocialAlreadyBound) {
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
	owner, err := systemservice.SocialSvcApp.FindOwnerUserID(ctx, socialID)
	if err != nil {
		if errors.Is(err, systemservice.ErrSocialNotFound) {
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

	ok, err := systemservice.SocialSvcApp.DeleteWithValidById(ctx, socialID)
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
