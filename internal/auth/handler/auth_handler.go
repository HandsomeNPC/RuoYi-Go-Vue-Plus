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
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/response"
)

// AuthHandler 认证接口，对应原项目 AuthController。
type AuthHandler struct {
	svc *service.AuthService

	// tokenHeader / tokenPrefix 取 token 用。
	//
	// 在构造时从配置捕获，**不在每请求里调 config.Get()** —— 那个取值器会加锁
	// 且未初始化时 panic，本包的一贯纪律是只在启动期读一次配置
	// （对齐 pkg/middleware 各中间件在 r.Use(...) 那一刻捕获配置的做法）。
	tokenHeader string
	tokenPrefix string
}

// NewAuthHandler 构造认证 handler。
//
// cfg 由 router 传入（那里已经拿着 *config.Config），本层不自己去取全局配置。
func NewAuthHandler(svc *service.AuthService, cfg config.Auth) *AuthHandler {
	header := cfg.Header
	if header == "" {
		header = config.TokenHeader
	}
	return &AuthHandler{
		svc:         svc,
		tokenHeader: header,
		// TokenPrefix 为空是有意义的取值（不使用前缀），不回落默认值 ——
		// 与鉴权中间件保持同一套语义，否则登出取到的 token 会与鉴权时的不一致。
		tokenPrefix: cfg.TokenPrefix,
	}
}

// Login 登录，POST /auth/login。
//
// 对应 AuthController.login。handler 只做三件事（对齐 CLAUDE.md 的分层约定）：
// 绑参数、调 service、返回 response.R —— 业务逻辑一律在 service。
//
// 绑定失败走 c.Error 交给 Recover 渲染，不自己拼响应体：
// 那样 400 参数错误与业务错误的响应形态才一致（HTTP 恒 200 + 业务码）。
func (h *AuthHandler) Login(c *gin.Context) {
	var body model.LoginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		// 校验失败的原始信息（validator 的字段路径）对前端没用也不好看，
		// 但对调试有用 —— 走 Detail 只进日志。
		_ = c.Error(errs.NewCode(response.CodeBadRequest, "参数校验失败").
			WithDetail(err.Error()))
		return
	}

	// 客户端 IP 由中间件层的取值逻辑统一提供（复用 X-Forwarded-For 那套
	// 头顺序），不在这里读 c.ClientIP() —— gin 那个的信任策略与
	// 原项目的头顺序不同，两处并存会让登录日志和 IP 白名单看到不同的 IP。
	vo, err := h.svc.Login(c.Request.Context(), &body, middleware.ClientIP(c.Request))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(vo))
}

// Logout 登出，POST /auth/logout。
//
// 对应 AuthController.logout。
//
// # 为什么自己从请求头取 token
//
// /auth/** 在免鉴权名单里（见 config.defaultAuthExcludes），所以鉴权中间件
// 不会跑，c 里也就没有 LoginUser —— 本方法要自己拿 token。
//
// 这是对齐 Java 的：那边 AuthController 整个类挂 @SaIgnore，
// logout 同样不要求有效 token，且 SysLoginService.logout 吞掉
// NotLoginException。理由是登出必须**总能成功** ——
// 让一个已过期的 token 无法登出，前端就没有干净的方式清理本地状态。
func (h *AuthHandler) Logout(c *gin.Context) {
	token := auth.TrimTokenPrefix(c.GetHeader(h.tokenHeader), h.tokenPrefix)

	if err := h.svc.Logout(c.Request.Context(), token); err != nil {
		_ = c.Error(err)
		return
	}

	// 文案对齐 Java 的 R.ok("退出成功")。
	c.JSON(http.StatusOK, response.OkMsg("退出成功"))
}
