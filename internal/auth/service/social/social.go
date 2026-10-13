// Package social 持有 auth 进程的三方登录管理器。
//
// ruoyi-common-go/oauth2 是实例式的(Manager + 注入的 StateStore)，而 handler /
// service 层按本项目惯例走包级函数调用；此处收下那个实例，避免每个调用点都传。
package social

import (
	"context"
	"sync/atomic"

	goredis "github.com/redis/go-redis/v9"

	"github.com/HandsomeNPC/ruoyi-common-go/oauth2"
	sysloginservice "ruoyi-go-vue-plus/internal/auth/service/syslogin"
)

// manager 由 Init 写入。用 atomic.Pointer 而非裸变量：
// 读发生在 handler goroutine，写只在启动期，但 go vet -race 下裸变量仍算竞态。
var manager atomic.Pointer[oauth2.Manager]

// Init 构造三方登录管理器。rdb 为 state 存储所用的 Redis 客户端，
// 须在 redis.Init 之后传入 redis.Client()——多进程部署下授权跳转与回调
// 可能落在不同进程，state 只能存 Redis，不能用 oauth2.NewMemoryStore。
func Init(cfg oauth2.Config, rdb *goredis.Client) error {
	m, err := oauth2.New(cfg, oauth2.NewRedisStore(rdb, sysloginservice.SocialAuthCodeKey))
	if err != nil {
		return err
	}
	manager.Store(m)
	return nil
}

// AuthorizeURL 生成授权跳转地址。
func AuthorizeURL(ctx context.Context, source string) (string, error) {
	m := manager.Load()
	if m == nil {
		return "", oauth2.ErrUnsupportedSource
	}
	return m.AuthorizeURL(ctx, source)
}

// Callback 执行三方登录回调，换回三方用户资料。
func Callback(ctx context.Context, source, code, state string) (*oauth2.AuthUser, error) {
	m := manager.Load()
	if m == nil {
		return nil, oauth2.ErrUnsupportedSource
	}
	return m.Callback(ctx, source, code, state)
}
