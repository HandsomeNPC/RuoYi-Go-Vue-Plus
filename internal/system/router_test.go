package system

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/memory"

	"ruoyi-go-vue-plus/pkg/config"
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

// TestRegisterRoutesDictPaths 字典数据与字典类型的十三个接口都已按 Java
// SysDictDataController / SysDictTypeController 的方法与路径注册到真实路由表上。
func TestRegisterRoutesDictPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /dict/data/list",
		"GET /dict/data/type/:dictType",
		"GET /dict/data/:dictCode",
		"POST /dict/data/export",
		"POST /dict/data",
		"PUT /dict/data",
		"DELETE /dict/data/:dictCodes",
		"GET /dict/type/list",
		"GET /dict/type/optionselect",
		"GET /dict/type/:dictId",
		"POST /dict/type/export",
		"POST /dict/type",
		"PUT /dict/type",
		"DELETE /dict/type/refreshCache",
		"DELETE /dict/type/:dictIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}

// TestRegisterRoutesMenuPaths 菜单的八个接口都已按 Java SysMenuController 的
// 方法与路径注册到真实路由表上。
func TestRegisterRoutesMenuPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /menu/getRouters",
		"GET /menu/list",
		"GET /menu/treeselect",
		"GET /menu/roleMenuTreeselect/:roleId",
		"GET /menu/:menuId",
		"POST /menu",
		"PUT /menu",
		"DELETE /menu/cascade/:menuIds",
		"DELETE /menu/:menuId",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
}

// TestRegisterRoutesNoticePaths 通知公告的五个接口都已按 Java SysNoticeController 的
// 方法与路径注册到真实路由表上。Java 侧没有导出接口，故不该出现 /notice/export。
func TestRegisterRoutesNoticePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	r := gin.New()
	RegisterRoutes(r, "")

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	for _, want := range []string{
		"GET /notice/list",
		"GET /notice/:noticeId",
		"POST /notice",
		"PUT /notice",
		"DELETE /notice/:noticeIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
	if registered["POST /notice/export"] {
		t.Error("Java 侧 notice 无导出接口，不应注册 POST /notice/export")
	}
}

// TestRegisterResourceRoutes 消息盒子与推送端点注册在 /resource 下且不带模块前缀
// （对齐 Java：SysMessageController 挂 /resource/message，推送端点挂 message.path）。
//
// 有意加载真实 configs/*.yaml：推送路径取自配置，一旦 yaml 里的 push.path
// 被改坏，前端连不上而后端毫无报错——那种故障只能靠这条断言提前暴露。
func TestRegisterResourceRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")

	r := gin.New()
	RegisterResourceRoutes(r)

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	if !registered["GET /resource/message/box"] {
		t.Error("未注册路由 GET /resource/message/box")
	}

	cfg := config.Get().Push
	if !cfg.Enabled {
		t.Skip("push.enabled=false，推送端点按设计不注册")
	}
	for _, want := range []string{"GET " + cfg.Path, "GET " + cfg.Path + "/close"} {
		if !registered[want] {
			t.Errorf("未注册推送路由 %s", want)
		}
	}
}

// TestGinStaticSegmentsBeatWildcards 钉住 RegisterRoutes 依赖的 gin 路由规则：
// 同层的静态段优先于通配段，故 /config/configKey/...、/config/refreshCache
// 与 /dept/optionselect 不会被邻居 /config/:configId、/dept/:deptId 抢走；
// 并列静态子树（/dict/data 与 /dict/type）也互不串台。
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

	// 字典最易走错：/dict/data 与 /dict/type 是 /dict 下并列静态段，而
	// /dict/data/type/:dictType 的第三段又叫 "type"——它不该被 /dict/type 的分支截走。
	dd := r.Group("/dict/data")
	dd.GET("/list", probe("dictDataList"))
	dd.GET("/type/:dictType", probe("dictDataByType"))
	dd.GET("/:dictCode", probe("dictDataInfo"))
	dt := r.Group("/dict/type")
	dt.GET("/list", probe("dictTypeList"))
	dt.GET("/optionselect", probe("dictTypeOptionSelect"))
	dt.GET("/:dictId", probe("dictTypeInfo"))
	dt.DELETE("/refreshCache", probe("dictRefreshCache"))
	dt.DELETE("/:dictIds", probe("dictTypeRemove"))

	// 菜单：DELETE /menu/cascade/:menuIds 与 DELETE /menu/:menuId 同层，
	// 前者段更具体。若被后者截走，级联删会退化成"删除主键为 cascade 的菜单"。
	m := r.Group("/menu")
	m.GET("/getRouters", probe("getRouters"))
	m.GET("/treeselect", probe("menuTreeSelect"))
	m.GET("/roleMenuTreeselect/:roleId", probe("roleMenuTreeSelect"))
	m.GET("/:menuId", probe("menuInfo"))
	m.DELETE("/cascade/:menuIds", probe("menuCascadeRemove"))
	m.DELETE("/:menuId", probe("menuRemove"))

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
		// 关键一组：data 与 type 两棵子树互不串台。
		{"GET", "/dict/data/type/sys_user_gender", "dictDataByType"},
		{"GET", "/dict/data/1761600000000000001", "dictDataInfo"},
		{"GET", "/dict/data/list", "dictDataList"},
		{"GET", "/dict/type/list", "dictTypeList"},
		{"GET", "/dict/type/optionselect", "dictTypeOptionSelect"},
		{"GET", "/dict/type/1761500000000000001", "dictTypeInfo"},
		{"DELETE", "/dict/type/refreshCache", "dictRefreshCache"},
		{"DELETE", "/dict/type/1,2,3", "dictTypeRemove"},
		{"GET", "/menu/getRouters", "getRouters"},
		{"GET", "/menu/treeselect", "menuTreeSelect"},
		{"GET", "/menu/roleMenuTreeselect/1761300000000000001", "roleMenuTreeSelect"},
		{"GET", "/menu/1761200000000000001", "menuInfo"},
		{"DELETE", "/menu/cascade/1,2,3", "menuCascadeRemove"},
		{"DELETE", "/menu/1761200000000000001", "menuRemove"},
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
