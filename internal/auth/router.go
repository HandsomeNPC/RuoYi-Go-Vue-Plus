// Package auth 认证模块(登录/登出/验证码)。
package auth

import (
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/auth/handler"
	"ruoyi-go-vue-plus/internal/auth/service"
	"ruoyi-go-vue-plus/pkg/config"
)

// Deps 本模块的外部依赖，由 cmd/auth 注入。
//
// 显式传依赖而非在模块内部调 database.DB() / redis.Client()：
// 那两个包级取值器未初始化时会 panic，把它们藏在业务代码里意味着
// 每个 service 都能在任意时刻 panic，且无法脱离全局状态测试。
type Deps struct {
	DB    *gorm.DB
	Redis goredis.UniversalClient
	Cfg   *config.Config
}

// RegisterRoutes 挂载认证模块路由，对应原项目 AuthController 的 @RequestMapping("/auth")。
//
// # 这些接口都在免鉴权名单里
//
// /auth/** 已配进 middleware.auth.excludes（见 config.defaultAuthExcludes），
// 对齐 Java 侧 AuthController 类上的 @SaIgnore。
// **漏了那条配置，登录接口自己就需要 token，谁也登不进来。**
//
// # 阶段 1 只有两个接口
//
// 原项目 AuthController 还有 register / social 相关接口，
// 依赖注册开关与三方登录（阶段 4），届时在这里加。
func RegisterRoutes(r gin.IRouter, deps Deps) {
	svc := service.NewAuthService(deps.DB, deps.Redis, deps.Cfg)
	h := handler.NewAuthHandler(svc, deps.Cfg.Middleware.Auth)

	g := r.Group("/auth")
	{
		g.POST("/login", h.Login)
		g.POST("/logout", h.Logout)
	}
}
