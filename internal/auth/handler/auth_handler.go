package handler

import (
	"log"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"ruoyi-go-vue-plus/internal/auth/model"
	authservice "ruoyi-go-vue-plus/internal/auth/service"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
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

// Logout 登出。
func (a *AuthApi) Logout(c *gin.Context) {
	// TODO(阶段 3): 登出逻辑待重建（原 AuthService.Logout 已删，待基于会话/在线记录重写）。
	c.JSON(http.StatusOK, response.OkMsg("退出成功"))
}
