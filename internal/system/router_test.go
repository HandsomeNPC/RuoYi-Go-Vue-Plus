package system

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/memory"
)

// setupManager 装一个内存态 sa-token Manager。
// RegisterRoutes 在构造中间件时就会取全局 Manager，不装则直接 panic；
// 换内存 storage 是为了不依赖 Redis——本文件只验路由表形状，不验鉴权行为。
func setupManager(t *testing.T) {
	t.Helper()
	sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).Build())
}

// TestRegisterRoutesConfigPaths 参数配置的九个接口都已按 Java SysConfigController 的
// 方法与路径注册到真实路由表上（而非另建一份探针，那只能验 gin 的规则、验不到本文件的注册）。
func TestRegisterRoutesConfigPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /config/list",
		"GET /config/:configId",
		"GET /config/configKey/:configKey",
		"POST /config/export",
		"POST /config",
		"PUT /config",
		"PUT /config/updateByKey",
		"DELETE /config/refreshCache",
		"DELETE /config/:configIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}

// TestRegisterRoutesDeptPaths 部门的七个接口都已按 Java SysDeptController 的
// 方法与路径注册到真实路由表上。
func TestRegisterRoutesDeptPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /dept/list",
		"GET /dept/list/exclude/:deptId",
		"GET /dept/optionselect",
		"GET /dept/:deptId",
		"POST /dept",
		"PUT /dept",
		"DELETE /dept/:deptId",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}

// TestGinStaticSegmentsBeatWildcards 钉住 RegisterRoutes 依赖的 gin 路由规则：
// 同层的静态段优先于通配段，故 /config/configKey/...、/config/refreshCache
// 与 /dept/optionselect 不会被邻居 /config/:configId、/dept/:deptId 抢走。
//
// 这条规则一旦变化，DELETE /config/refreshCache 会被当成"删除主键为 refreshCache
// 的配置"、GET /dept/optionselect 会被当成"查主键为 optionselect 的部门"而
// 静默走错分支——不报错，只是行为错，值得单独钉一条。
//
// 用探针 engine 而非 RegisterRoutes：后者每条路由都挂了鉴权中间件，
// 发真实请求会先撞上登录态与 Redis。真实注册的形状由
// TestRegisterRoutesConfigPaths 负责，这里只验 gin 的解析优先级。
func TestGinStaticSegmentsBeatWildcards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	probe := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { c.String(http.StatusOK, name) }
	}

	// 注册顺序与 RegisterRoutes 一致。
	g := r.Group("/config")
	g.GET("/configKey/:configKey", probe("getByKey"))
	g.GET("/:configId", probe("getInfo"))
	g.DELETE("/refreshCache", probe("refreshCache"))
	g.DELETE("/:configIds", probe("remove"))

	// 部门侧同理：/dept/optionselect 与 /dept/list/exclude/:deptId 都得躲开 /dept/:deptId。
	d := r.Group("/dept")
	d.GET("/list", probe("deptList"))
	d.GET("/list/exclude/:deptId", probe("excludeChild"))
	d.GET("/optionselect", probe("optionselect"))
	d.GET("/:deptId", probe("deptInfo"))

	tests := []struct {
		method, path, want string
	}{
		{"GET", "/config/configKey/sys.user.initPassword", "getByKey"},
		{"GET", "/config/1761700000000000001", "getInfo"},
		{"DELETE", "/config/refreshCache", "refreshCache"},
		{"DELETE", "/config/1,2,3", "remove"},
		{"GET", "/dept/list", "deptList"},
		{"GET", "/dept/list/exclude/1761000000000000100", "excludeChild"},
		{"GET", "/dept/optionselect", "optionselect"},
		{"GET", "/dept/1761000000000000100", "deptInfo"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("%s %s -> %d, 期望 200", tt.method, tt.path, w.Code)
			}
			if got := w.Body.String(); got != tt.want {
				t.Errorf("%s %s 命中 %q, 期望 %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
