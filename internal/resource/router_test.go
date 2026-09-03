package resource

import (
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

// registeredRoutes 注册真实路由并返回 "METHOD /path" 集合。
func registeredRoutes(t *testing.T, prefix string) map[string]bool {
	t.Helper()
	gin.SetMode(gin.TestMode)
	setupManager(t)

	r := gin.New()
	RegisterRoutes(r, prefix)

	out := make(map[string]bool)
	for _, ri := range r.Routes() {
		out[ri.Method+" "+ri.Path] = true
	}
	return out
}

// TestRegisterRoutesOssPaths OSS 的五个接口按 Java SysOssController 的方法与路径
// 注册到真实路由表上（而非另建一份探针，那只能验 gin 的规则、验不到本文件的注册）。
func TestRegisterRoutesOssPaths(t *testing.T) {
	registered := registeredRoutes(t, RoutePrefix)

	for _, want := range []string{
		"GET /resource/oss/list",
		"GET /resource/oss/listByIds/:ossIds",
		"GET /resource/oss/download/:ossId",
		"POST /resource/oss/upload",
		"DELETE /resource/oss/:ossIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
	// Java 侧 oss 无导出接口，不该凭空造一个。
	if registered["POST /resource/oss/export"] {
		t.Error("Java 侧 oss 无导出接口，不应注册 POST /resource/oss/export")
	}
}

// TestRegisterRoutesOssConfigPaths 对象存储配置的六个接口都已注册。
func TestRegisterRoutesOssConfigPaths(t *testing.T) {
	registered := registeredRoutes(t, RoutePrefix)

	for _, want := range []string{
		"GET /resource/oss/config/list",
		"GET /resource/oss/config/:ossConfigId",
		"POST /resource/oss/config",
		"PUT /resource/oss/config",
		"PUT /resource/oss/config/changeStatus",
		"DELETE /resource/oss/config/:ossConfigIds",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
	// 根路径须是 "" 而非 "/"：后者会注册成 /resource/oss/config/，前端打不中。
	if registered["POST /resource/oss/config/"] {
		t.Error("根路径应注册为 /resource/oss/config 而非带尾斜杠")
	}
}

// TestOssConfigAndOssIDsCoexist /oss/config 与同层通配 /oss/:ossIds 必须能共存。
//
// 这条一旦破，DELETE /resource/oss/config/1 会被当成"删除 ossIds=config"而
// 静默走错分支——把配置删除请求变成文件删除请求，且不报任何错。
func TestOssConfigAndOssIDsCoexist(t *testing.T) {
	registered := registeredRoutes(t, RoutePrefix)

	if !registered["DELETE /resource/oss/config/:ossConfigIds"] {
		t.Fatal("DELETE /resource/oss/config/:ossConfigIds 未注册，配置删除会落到 /oss/:ossIds")
	}
	if !registered["DELETE /resource/oss/:ossIds"] {
		t.Error("DELETE /resource/oss/:ossIds 未注册")
	}
	// 同理，GET 侧的 /oss/list 与 /oss/config/list 也得各自成立。
	if !registered["GET /resource/oss/list"] || !registered["GET /resource/oss/config/list"] {
		t.Error("/oss/list 与 /oss/config/list 应同时注册")
	}
}

// TestRegisterRoutesMessageBox 消息盒子随 /resource 一起归本模块
// （Java 的 SysMessageController 挂 /resource/message，与 OSS 同前缀）。
func TestRegisterRoutesMessageBox(t *testing.T) {
	registered := registeredRoutes(t, RoutePrefix)

	if !registered["GET /resource/message/box"] {
		t.Error("未注册路由 GET /resource/message/box")
	}
	if !registered["GET /resource/ping"] {
		t.Error("未注册探针 GET /resource/ping")
	}
}

// TestRegisterRoutesCaptchaPaths 短信与邮箱验证码端点已注册
// （对齐 Java CaptchaController 的 /resource/sms/code 与 /resource/email/code）。
//
// 图形验证码 /auth/code 归 auth 模块，不该在本模块出现。
func TestRegisterRoutesCaptchaPaths(t *testing.T) {
	registered := registeredRoutes(t, RoutePrefix)

	for _, want := range []string{
		"GET /resource/sms/code",
		"GET /resource/email/code",
	} {
		if !registered[want] {
			t.Errorf("未注册路由 %s", want)
		}
	}
	if registered["GET /resource/auth/code"] || registered["GET /resource/code"] {
		t.Error("图形验证码属 auth 模块，不应在 resource 注册")
	}
}

// TestRegisterRoutesPrefixApplied 前缀由调用方传入，不写死在路由里。
// 传空即得到裸路径，传 /resource 才有模块段——这条保证 prefix 参数真的生效。
func TestRegisterRoutesPrefixApplied(t *testing.T) {
	bare := registeredRoutes(t, "")
	if !bare["GET /oss/list"] {
		t.Error("prefix 传空时应注册裸路径 GET /oss/list")
	}
	if bare["GET /resource/oss/list"] {
		t.Error("prefix 传空时不该出现 /resource 段，说明前缀被写死在路由里")
	}

	prefixed := registeredRoutes(t, RoutePrefix)
	if !prefixed["GET /resource/oss/list"] {
		t.Error("prefix 传 /resource 时应注册 GET /resource/oss/list")
	}
	if prefixed["GET /resource/resource/oss/list"] {
		t.Error("路由被拼了双前缀")
	}
}

// TestRegisterPushRoutes 推送端点按配置路径注册。
//
// 有意加载真实 configs/*.yaml：推送路径取自配置，一旦 yaml 里的 push.path
// 被改坏，前端连不上而后端毫无报错——那种故障只能靠这条断言提前暴露。
func TestRegisterPushRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManager(t)
	config.Load("../../configs/application.yaml", "../../configs/resource.yaml")

	r := gin.New()
	RegisterPushRoutes(r)

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
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
