package monitor

import (
	"log"

	"github.com/gin-gonic/gin"

	monitorconfig "ruoyi-go-vue-plus/internal/monitor/config"
)

// InitServer 启动 HTTP 服务并阻塞。
func InitServer(r *gin.Engine) {
	cfg := monitorconfig.Get().Server
	log.Printf("[%s] 监听 %s", cfg.Name, cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
