package middleware

import (
	"github.com/gin-gonic/gin"
)

// Auth 鉴权中间件。
//
// TODO: 待 pkg/auth 的登录态原语（Claims / Sign / Verify / SessionStore）重新实现后补全。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
