// Package auth 认证模块(登录/登出/验证码)。
package auth

import (
	"log"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// InitServer 启动 HTTP 服务并阻塞
func InitServer(r *gin.Engine) {
	cfg := config.Get()
	log.Printf("[%s] 监听 %s", cfg.Server.Name, cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		log.Fatal(err)
	}
}
