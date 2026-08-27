// Package satoken 封装 sa-token-go 的初始化与全局访问。
package satoken

import (
	"log"

	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/redis"

	"ruoyi-go-vue-plus/pkg/config"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
)

// Init 按 config.SATokenConfig 与包级 Redis 客户端构建全局 sa-token Manager。
func Init() {
	c := config.Get()
	cfg := c.SAToken

	storage := redis.NewStorageFromClient(pkgredis.Client())

	manager := sagin.NewBuilder().
		Storage(storage).
		TokenName(cfg.TokenName).        // 可配
		IsConcurrent(cfg.IsConcurrent).  // 可配
		IsShare(cfg.IsShare).            // 可配
		TokenStyle(sagin.TokenStyleJWT). // 固定 JWT
		JwtSecretKey(cfg.JwtSecretKey).  // 可配
		IsReadCookie(true).              // 固定：启用 Cookie 读 token
		IsReadBody(true).                // 固定：启用 Body 读 token
		Build()

	sagin.SetManager(manager)
	log.Printf("[%s] sa-token 已就绪: tokenName=%s",
		c.Server.Name, cfg.TokenName)
}

// Manager 返回全局 Manager，未调用 Init 会由 sagin.GetManager 自身 panic。
func Manager() *sagin.Manager {
	return sagin.GetManager()
}
