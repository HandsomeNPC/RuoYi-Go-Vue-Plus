package auth

import (
	"testing"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/memory"
)

// setupManager 装一个内存态 sa-token Manager。
// RegisterRoutes 构造中间件时就会取全局 Manager，不装则直接 panic；
// 用内存 storage 是为了不依赖 Redis——本文件只验路由表形状，不验鉴权行为。
func setupManager(t *testing.T) {
	t.Helper()
	sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).Build())
}

// TestRegisterRoutesPaths auth 的接口都已按 Java AuthController 的方法与路径
// 注册到真实路由表上（而非另建一份探针，那只能验 gin 的规则、验不到本文件的注册）。
func TestRegisterRoutesPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"POST /login",
		"POST /logout",
		"GET /code",
		// 三方绑定三件套。路径与 HTTP 方法逐字对齐 Java AuthController：
		// 前端 RuoYi-Plus-UI 的 api/system/social/auth.ts 按字面量调用，
		// 方法写错(如 unlock 用 POST)前端不会报错，只会静默 404。
		"GET /binding/:source",
		"POST /social/callback",
		"DELETE /unlock/:socialId",
	} {
		if !registered[want] {
			t.Errorf("路由 %q 未注册", want)
		}
	}
}

// TestRegisterRoutesWithPrefix standalone 部署带 /auth 前缀，
// 与 modular 经 nginx 剥前缀后的无前缀形态须同时成立。
func TestRegisterRoutesWithPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "/auth")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /auth/binding/:source",
		"POST /auth/social/callback",
		"DELETE /auth/unlock/:socialId",
	} {
		if !registered[want] {
			t.Errorf("路由 %q 未注册", want)
		}
	}
}
