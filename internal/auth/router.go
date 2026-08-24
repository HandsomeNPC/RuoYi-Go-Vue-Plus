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
// 配置不在此列：它由 config.Load 写入包级实例，这里直接 config.Get() 取，
// 不必让 main 逐层传参（与 pkg/middleware 各中间件读配置的方式一致）。
type Deps struct {
	DB    *gorm.DB
	Redis goredis.UniversalClient
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
//
// 读 config.Get()，故 **必须在 config.Load 之后调用**，否则 panic。
func RegisterRoutes(r gin.IRouter, deps Deps) {
	cfg := config.Get()
	svc := service.NewAuthService(deps.DB, deps.Redis, cfg)
	h := handler.NewAuthHandler(svc, cfg.Middleware.Auth)

	g := r.Group("/auth")
	{
		g.POST("/login", h.Login)
		g.POST("/logout", h.Logout)
	}
}
