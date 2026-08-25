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
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
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
	// 对应 Java @RequestBody String body：读原始 JSON 字节，供具体策略自行解析。
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
	// 授权类型和客户端id
	clientID := body.ClientID
	grantType := body.GrantType
	// 对应 Java: SysClientVo client = clientService.queryByClientId(clientId);
	client, err := systemservice.ClientSvcApp.QueryByClientID(c.Request.Context(), clientID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 查询不到 client 或 client 内不包含 grantType
	// （精确比对，非子串匹配；Java 用 StringUtils.contains 是子串匹配，这里刻意更严）
	if grantType == "" || !slices.Contains(client.GrantTypeList, grantType) {
		log.Printf("[auth] 客户端id: %s 认证类型: %s 异常", clientID, grantType)
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), "auth.grant.type.error"), ""))
		return
	} else if client.Status != constant.StatusNormal {
		// 客户端已停用
		_ = c.Error(errs.New(0, i18n.Msg(c.Request.Context(), "auth.grant.type.blocked"), ""))
		return
	}
	// 登录（client 已就绪，对应 Java IAuthStrategy.login(body, client, grantType)，不带 IP）。
	// 透传 *http.Request，service 内部用 req.Context() 取 ctx、ip.ClientIP(req) 取 IP。
	vo, err := authservice.AuthSvcApp.Login(c.Request, raw, grantType, client)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}

// Logout 登出。
func (a *AuthApi) Logout(c *gin.Context) {
	cfg := config.Get().Middleware.Auth
	header := cfg.Header
	if header == "" {
		header = config.TokenHeader
	}
	token := auth.TrimTokenPrefix(c.GetHeader(header), cfg.TokenPrefix)
	if err := authservice.AuthSvcApp.Logout(c.Request.Context(), token); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("退出成功"))
}
