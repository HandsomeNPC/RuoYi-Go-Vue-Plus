package handler

import (
	"log"
	"net/http"
	"slices"

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
)

// AuthApi 认证接口。
type AuthApi struct{}

// AuthApiApp 包级实例。
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

// Logout 登出，对照 Java AuthController.logout。
// 与 Java 一样恒返回成功：service 内部吞掉未登录/token 失效的情况，前端总能正常清态。
func (a *AuthApi) Logout(c *gin.Context) {
	// token 由组级 TokenInterceptor 解析后存入 ctx；service 层不碰 *gin.Context，故在此取出传入。
	authservice.SysLoginSvcApp.Logout(c.Request, sagin.GetTokenFromCtx(c))

	c.JSON(http.StatusOK, response.OkMsg(i18n.Msg(c.Request.Context(), "user.logout.success")))
}

// Code 获取图形验证码，对照 Java CaptchaController.getCode。
func (a *AuthApi) Code(c *gin.Context) {
	vo, err := captcha.Generate(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}
