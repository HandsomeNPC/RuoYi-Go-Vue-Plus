// Command system 系统管理模块进程入口，默认监听 :8081。
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// TODO: config.Load() -> database.New() -> redis.New()
	// TODO: middleware 注入 -> system.RegisterRoutes(r, deps)

	r := gin.Default()
	r.GET("/system/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "system", "message": "pong"})
	})
	_ = r.Run(":8081")
}
