// Command auth 认证模块进程入口，默认监听 :8080。
//
// in-process 复用 system 的 service，故也连接同一数据库。
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// TODO: config.Load() -> database.New() -> redis.New()
	// TODO: middleware 注入 -> auth.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "auth", "message": "pong"})
	})
	_ = r.Run(":8080")
}
