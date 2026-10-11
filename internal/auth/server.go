package auth

import (
	"log"

	"github.com/gin-gonic/gin"

	authconfig "ruoyi-go-vue-plus/internal/auth/config"
)

// InitServer 启动 HTTP 服务并阻塞。
func InitServer(r *gin.Engine) {
	cfg := authconfig.Get().Server
	log.Printf("[%s] 监听 %s", cfg.Name, cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
