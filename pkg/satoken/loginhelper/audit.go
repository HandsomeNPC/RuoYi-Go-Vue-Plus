package loginhelper

import (
	"github.com/gin-gonic/gin"

	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// AuditContext 把登录用户的 userId/deptId 写进 request context，供 pkg/repository
// 的审计回调填 create_by/create_dept 等列。
//
// 之所以要这道中间件：GORM 回调只拿得到 *gorm.DB，取不到 *gin.Context，而 pkg/repository
// 也不能反向 import 本包（会成环），故登录态只能经 ctx 接力。等价于 Java 侧 sa-token
// 的 thread-local——那边由全局 servlet filter 自动持有，Go 无等价物只能显式挂载。
//
// 须注册在 TokenInterceptor 之后：GetLoginUser 读的 token 由它写进 gin.Context。
// 未登录时不写，由回调按 -1 兜底。
func AuditContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只调一次 GetLoginUser：它要读 Redis，用 GetUserID+GetDeptID 会变成两次往返。
		if lu := GetLoginUser(c); lu != nil {
			c.Request = c.Request.WithContext(
				pkgrepo.WithAuditUser(c.Request.Context(), pkgrepo.AuditUser{
					UserID: lu.UserID,
					DeptID: lu.DeptID,
				}))
		}
		c.Next()
	}
}
