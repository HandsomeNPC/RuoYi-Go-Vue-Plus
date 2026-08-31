package satoken

import (
	"log"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// 鉴权失败提示，与 Java SaTokenExceptionHandler 的两类文案一致。
const (
	msgNotPermission = "没有访问权限，请联系管理员授权"
	msgNotLogin      = "登录状态异常，请重新登录"
)

// CheckPermission 权限码校验中间件，多个权限码任一命中即放行。
//
// 校验本身复用 sa-token（权限于登录时写入 account session），本层只做响应适配：
// sagin.CheckPermission 直接吐 gin.H 且 HTTP 状态非 200，与本项目 R + 恒 200 的契约不符，
// 故改用返回 error 的 ByToken 版本，交由 middleware.Recover 统一渲染。
//
// 语义为 OR（同 sagin.CheckPermission）。Java @SaCheckPermission 默认 AND，
// 但上游所有 Controller 都只传单个权限码，差异不可观测。
func CheckPermission(perms ...string) gin.HandlerFunc {
	return check(perms, sagin.CheckPermissionOrByToken, "权限码校验失败")
}

// CheckRole 角色校验中间件，多个角色任一命中即放行。
func CheckRole(roles ...string) gin.HandlerFunc {
	return check(roles, sagin.CheckRoleOrByToken, "角色权限校验失败")
}

// check 组装校验中间件。verify 失败即视为无权限，其内部区分不了的未登录场景由 token 空分支兜住。
func check(want []string, verify func(string, []string) error, reason string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// token 由组级 TokenInterceptor 解析后写入 ctx，未挂该中间件时恒为空。
		token := sagin.GetTokenFromCtx(c)
		if token == "" {
			_ = c.Error(errs.New(response.CodeUnauthorized, msgNotLogin, "请求未携带 token"))
			c.Abort()
			return
		}
		if err := verify(token, want); err != nil {
			log.Printf("[satoken] 请求地址'%s',%s'%v': %v", c.Request.URL.Path, reason, want, err)
			_ = c.Error(errs.New(response.CodeForbidden, msgNotPermission, err.Error()))
			c.Abort()
			return
		}
		c.Next()
	}
}
