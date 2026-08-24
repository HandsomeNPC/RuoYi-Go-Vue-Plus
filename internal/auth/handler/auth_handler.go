// Package handler auth 模块 HTTP 层：登录、登出、验证码等接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/auth/model"
	"ruoyi-go-vue-plus/internal/auth/service"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/response"
)

// AuthApi 认证接口
type AuthApi struct{}

// AuthApiApp 包级实例，router 直接 handler.AuthApiApp.Login / .Logout 挂路由。
var AuthApiApp = new(AuthApi)

// Login 登录
func (a *AuthApi) Login(c *gin.Context) {
	var body model.LoginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	vo, err := service.AuthSvcApp.Login(c.Request.Context(), &body, ip.ClientIP(c.Request))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}

// Logout 登出
func (a *AuthApi) Logout(c *gin.Context) {
	cfg := config.Get().Middleware.Auth
	header := cfg.Header
	if header == "" {
		header = config.TokenHeader
	}
	token := auth.TrimTokenPrefix(c.GetHeader(header), cfg.TokenPrefix)
	if err := service.AuthSvcApp.Logout(c.Request.Context(), token); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkMsg("退出成功"))
}
