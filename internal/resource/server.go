package resource

import (
	"log"

	"github.com/gin-gonic/gin"

	resourceconfig "ruoyi-go-vue-plus/internal/resource/config"
)

// InitServer 启动 HTTP 服务并阻塞。
func InitServer(r *gin.Engine) {
	cfg := resourceconfig.Get().Server
	log.Printf("[%s] 监听 %s", cfg.Name, cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
