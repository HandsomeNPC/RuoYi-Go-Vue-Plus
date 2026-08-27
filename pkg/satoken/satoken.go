// Package satoken 封装 sa-token-go 的初始化与全局访问。
//
// 多进程架构(auth :8080 / system :8081)共用同一个 Redis，登录态存在 Redis 里，
// 因此每个进程启动时都要各自调用 Init 指向同一个 Redis 客户端与同一份配置，
// 这样 auth 进程签发的 token 在 system 进程的中间件里能验过。
package satoken

import (
	"log"

	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/redis"

	"ruoyi-go-vue-plus/pkg/config"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
)

// Init 按 config.SATokenConfig 与包级 Redis 客户端构建全局 sa-token Manager。
//
// 必须在 config.Load 与 redis.Init 之后调用。重复调用会覆盖前一个 Manager
// （多进程各自只调一次即可）。Manager 一经 SetManager 即全局生效，
// sagin 的登录/鉴权 API 与 gin 中间件都从全局 Manager 取。
func Init() {
	c := config.Get()
	cfg := c.SAToken

	storage := redis.NewStorageFromClient(pkgredis.Client())

	b := sagin.NewBuilder().
		Storage(storage).
		TokenName(cfg.TokenName).
		Timeout(cfg.Timeout).
		IsConcurrent(cfg.IsConcurrent).
		IsShare(cfg.IsShare).
		TokenStyle(mapTokenStyle(cfg.TokenStyle)).
		IsReadHeader(cfg.IsReadHeader).
		IsReadCookie(cfg.IsReadCookie).
		IsReadBody(cfg.IsReadBody).
		AutoRenew(cfg.AutoRenew).
		KeyPrefix(cfg.KeyPrefix).
		IsLog(cfg.IsLog).
		IsPrintBanner(cfg.IsPrintBanner)

	if cfg.ActiveTimeout > 0 {
		b = b.ActiveTimeout(cfg.ActiveTimeout)
	} else {
		b = b.NoActiveTimeout()
	}
	if cfg.TokenStyle == "jwt" {
		b = b.JwtSecretKey(cfg.JwtSecretKey)
	}

	sagin.SetManager(b.Build())
	log.Printf("[%s] sa-token 已就绪: tokenName=%s style=%s timeout=%ds prefix=%s",
		c.Server.Name, cfg.TokenName, cfg.TokenStyle, cfg.Timeout, cfg.KeyPrefix)
}

// Manager 返回全局 Manager，未调用 Init 会由 sagin.GetManager 自身 panic。
func Manager() *sagin.Manager {
	return sagin.GetManager()
}

// mapTokenStyle 把配置字符串映射成 sa-token 的 TokenStyle 常量。
// 未识别的值回落到 uuid（与 Builder 默认一致）。
func mapTokenStyle(s string) sagin.TokenStyle {
	switch s {
	case "", "uuid":
		return sagin.TokenStyleUUID
	case "simple":
		return sagin.TokenStyleSimple
	case "random-32":
		return sagin.TokenStyleRandom32
	case "random-64":
		return sagin.TokenStyleRandom64
	case "random-128":
		return sagin.TokenStyleRandom128
	case "jwt":
		return sagin.TokenStyleJWT
	case "hash":
		return sagin.TokenStyleHash
	case "timestamp":
		return sagin.TokenStyleTimestamp
	case "tik":
		return sagin.TokenStyleTik
	default:
		return sagin.TokenStyleUUID
	}
}
