package monitor

import (
	"testing"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/memory"
)

// setupManager 装一个内存态 sa-token Manager。
// RegisterRoutes 构造中间件时就会取全局 Manager，不装则直接 panic；
// 换内存 storage 是为了不依赖 Redis——本文件只验路由表形状，不验鉴权行为。
func setupManager(t *testing.T) {
	t.Helper()
	sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).Build())
}

// TestRegisterRoutesLoginInfoPaths 登录日志的五个接口都已按 Java SysLoginInfoController
// 的方法与路径注册到真实路由表上。重点钉住 DELETE /loginInfo/clean 与
// DELETE /loginInfo/:infoIds 同层共存——gin 静态段优先，clean 不会被当成主键。
// 一旦 gin 的该规则变化，clean 会静默走错分支，故值得测试。
func TestRegisterRoutesLoginInfoPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /loginInfo/list",
		"POST /loginInfo/export",
		"DELETE /loginInfo/clean",
		"DELETE /loginInfo/:infoIds",
		"GET /loginInfo/unlock/:userName",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}

// TestRegisterRoutesOperLogPaths 操作日志的四个接口都已按 Java SysOperlogController
// 的方法与路径注册到真实路由表上。重点钉住 DELETE /operlog/clean 与
// DELETE /operlog/:operIds 同层共存——gin 静态段优先，clean 不会被当成主键。
func TestRegisterRoutesOperLogPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /operlog/list",
		"POST /operlog/export",
		"DELETE /operlog/clean",
		"DELETE /operlog/:operIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}
