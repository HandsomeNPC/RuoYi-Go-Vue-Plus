package system

import (
	"log"

	"github.com/gin-gonic/gin"

	systemconfig "ruoyi-go-vue-plus/internal/system/config"
)

// InitServer 启动 HTTP 服务并阻塞。
func InitServer(r *gin.Engine) {
	cfg := systemconfig.Get().Server
	log.Printf("[%s] 监听 %s", cfg.Name, cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
