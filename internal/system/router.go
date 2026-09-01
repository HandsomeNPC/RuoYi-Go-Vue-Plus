package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/system/handler"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/repeatsubmit"
	"ruoyi-go-vue-plus/pkg/satoken"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// clientLogTitle 客户端管理的操作日志模块名，对照 Java @Log(title = "客户端管理")。
const clientLogTitle = "客户端管理"

// configLogTitle 参数管理的操作日志模块名，对照 Java @Log(title = "参数管理")。
const configLogTitle = "参数管理"

// deptLogTitle 部门管理的操作日志模块名，对照 Java @Log(title = "部门管理")。
const deptLogTitle = "部门管理"

// dictDataLogTitle 字典数据的操作日志模块名，对照 Java @Log(title = "字典数据")。
const dictDataLogTitle = "字典数据"

// dictTypeLogTitle 字典类型的操作日志模块名，对照 Java @Log(title = "字典类型")。
const dictTypeLogTitle = "字典类型"

// menuLogTitle 菜单管理的操作日志模块名，对照 Java @Log(title = "菜单管理")。
const menuLogTitle = "菜单管理"

func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())

	// 公开路由(免鉴权)：探针
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "system", "message": "pong"})
	})
	protected := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	protected.Use(plugin.TokenInterceptor(), loginhelper.AuditContext())
	user := protected.Group("/user")
	user.GET("/getInfo", sagin.CheckLogin(), handler.UserApiApp.GetInfo)

	menu := protected.Group("/menu")
	menu.GET("/getRouters", sagin.CheckLogin(), handler.MenuApiApp.GetRouters)
	// 多个中间件串联即 AND：Java 侧 list/getInfo 同时挂了 @SaCheckRole(超管)
	// 与 @SaCheckPermission，两道都得过。注意这与单个 CheckPermission 内部多值的
	// OR 语义不同——那是"任一权限码命中放行"。
	menu.GET("/list", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:list"), handler.MenuApiApp.List)
	// treeselect 与 roleMenuTreeselect 只要 query 权限，不挂超管角色（对照 Java）：
	// 角色授权界面要用它们，而分配角色的人未必是超管。
	menu.GET("/treeselect", satoken.CheckPermission("system:menu:query"),
		handler.MenuApiApp.TreeSelect)
	menu.GET("/roleMenuTreeselect/:roleId", satoken.CheckPermission("system:menu:query"),
		handler.MenuApiApp.RoleMenuTreeSelect)
	// getRouters/treeselect 等静态段与 :menuId 同层但静态段优先，无需调整注册顺序。
	menu.GET("/:menuId", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:query"), handler.MenuApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /menu/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	menu.POST("", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:add"),
		oplog.Log(menuLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.MenuApiApp.Add)
	menu.PUT("", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:edit"),
		oplog.Log(menuLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.MenuApiApp.Edit)
	// cascade/:menuIds 段更具体，与 /:menuId 可共存，静态段优先。
	menu.DELETE("/cascade/:menuIds", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:remove"),
		oplog.Log(menuLogTitle, enum.BusinessTypeDelete), handler.MenuApiApp.CascadeRemove)
	menu.DELETE("/:menuId", satoken.CheckRole(constant.SuperAdminRoleKey),
		satoken.CheckPermission("system:menu:remove"),
		oplog.Log(menuLogTitle, enum.BusinessTypeDelete), handler.MenuApiApp.Remove)

	client := protected.Group("/client")
	client.GET("/list", satoken.CheckPermission("system:client:list"), handler.ClientApiApp.List)
	client.GET("/:id", satoken.CheckPermission("system:client:query"), handler.ClientApiApp.GetInfo)
	// 导出走 POST 与 Java 一致：前端以 form 表单 POST 提交筛选条件。
	// 与 PUT ""/changeStatus 同理，路径更具体，须注册在 client.POST("") 之后。
	client.POST("/export", satoken.CheckPermission("system:client:export"),
		oplog.Log(clientLogTitle, enum.BusinessTypeExport), handler.ClientApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /client/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	client.POST("", satoken.CheckPermission("system:client:add"),
		oplog.Log(clientLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.ClientApiApp.Add)
	client.PUT("", satoken.CheckPermission("system:client:edit"),
		oplog.Log(clientLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ClientApiApp.Edit)
	// changeStatus 不挂防重：对齐 Java(仅 edit 带 @RepeatSubmit)，
	// 且它幂等——重复提交同一状态无副作用。须注册在 PUT "" 之后，路径更具体。
	client.PUT("/changeStatus", satoken.CheckPermission("system:client:edit"),
		oplog.Log(clientLogTitle, enum.BusinessTypeUpdate),
		handler.ClientApiApp.ChangeStatus)
	client.DELETE("/:ids", satoken.CheckPermission("system:client:remove"),
		oplog.Log(clientLogTitle, enum.BusinessTypeDelete),
		handler.ClientApiApp.Remove)

	config := protected.Group("/config")
	config.GET("/list", satoken.CheckPermission("system:config:list"), handler.ConfigApiApp.List)
	// configKey 与 :configId 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 与 Java 一致不校验权限码，仅需登录：前端多处要读参数却未必有配置管理权限。
	config.GET("/configKey/:configKey", sagin.CheckLogin(), handler.ConfigApiApp.GetConfigKey)
	config.GET("/:configId", satoken.CheckPermission("system:config:query"),
		handler.ConfigApiApp.GetInfo)
	// 导出走 POST 与 Java 一致：前端以 form 表单 POST 提交筛选条件。
	config.POST("/export", satoken.CheckPermission("system:config:export"),
		oplog.Log(configLogTitle, enum.BusinessTypeExport), handler.ConfigApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /config/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	config.POST("", satoken.CheckPermission("system:config:add"),
		oplog.Log(configLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.Add)
	config.PUT("", satoken.CheckPermission("system:config:edit"),
		oplog.Log(configLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.Edit)
	config.PUT("/updateByKey", satoken.CheckPermission("system:config:edit"),
		oplog.Log(configLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.UpdateByKey)
	// refreshCache 与 :configIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh：对照 Java @SaCheckPermission("system:config:remove")。
	config.DELETE("/refreshCache", satoken.CheckPermission("system:config:remove"),
		oplog.Log(configLogTitle, enum.BusinessTypeClean),
		handler.ConfigApiApp.RefreshCache)
	config.DELETE("/:configIds", satoken.CheckPermission("system:config:remove"),
		oplog.Log(configLogTitle, enum.BusinessTypeDelete),
		handler.ConfigApiApp.Remove)

	dept := protected.Group("/dept")
	// optionselect、list/exclude/:deptId 与 /:deptId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	dept.GET("/list", satoken.CheckPermission("system:dept:list"), handler.DeptApiApp.List)
	dept.GET("/list/exclude/:deptId", satoken.CheckPermission("system:dept:list"),
		handler.DeptApiApp.ExcludeChild)
	dept.GET("/optionselect", satoken.CheckPermission("system:dept:query"),
		handler.DeptApiApp.OptionSelect)
	dept.GET("/:deptId", satoken.CheckPermission("system:dept:query"), handler.DeptApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /dept/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	dept.POST("", satoken.CheckPermission("system:dept:add"),
		oplog.Log(deptLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DeptApiApp.Add)
	dept.PUT("", satoken.CheckPermission("system:dept:edit"),
		oplog.Log(deptLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DeptApiApp.Edit)
	// 与 Java 一致只支持单删：部门有父子关系，批量删无法保证删除顺序。
	dept.DELETE("/:deptId", satoken.CheckPermission("system:dept:remove"),
		oplog.Log(deptLogTitle, enum.BusinessTypeDelete), handler.DeptApiApp.Remove)

	// 字典数据与字典类型是 /dict 下两个并列静态段，各自独立注册。
	// 权限码共用 system:dict:*（对照 Java 两个 Controller 的注解），非 dict:data/dict:type 分设。
	dictData := protected.Group("/dict/data")
	dictData.GET("/list", satoken.CheckPermission("system:dict:list"),
		handler.DictDataApiApp.List)
	// type/:dictType 与 :dictCode 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 与 Java 一致不校验权限码，仅需登录：前端 DictTag 到处渲染字典标签却未必有字典管理权限。
	dictData.GET("/type/:dictType", sagin.CheckLogin(), handler.DictDataApiApp.DictType)
	dictData.GET("/:dictCode", satoken.CheckPermission("system:dict:query"),
		handler.DictDataApiApp.GetInfo)
	// 导出走 POST 与 Java 一致：前端 commonExport 以 form 表单提交筛选条件。
	dictData.POST("/export", satoken.CheckPermission("system:dict:export"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeExport), handler.DictDataApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /dict/data/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	dictData.POST("", satoken.CheckPermission("system:dict:add"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictDataApiApp.Add)
	dictData.PUT("", satoken.CheckPermission("system:dict:edit"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictDataApiApp.Edit)
	dictData.DELETE("/:dictCodes", satoken.CheckPermission("system:dict:remove"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeDelete), handler.DictDataApiApp.Remove)

	dictType := protected.Group("/dict/type")
	dictType.GET("/list", satoken.CheckPermission("system:dict:list"),
		handler.DictTypeApiApp.List)
	// optionselect 与 :dictId 同层，静态段优先，无需调整注册顺序。
	// 与 Java 一致不校验权限码，仅需登录：前端选字典时未必有字典管理权限。
	dictType.GET("/optionselect", sagin.CheckLogin(), handler.DictTypeApiApp.OptionSelect)
	dictType.GET("/:dictId", satoken.CheckPermission("system:dict:query"),
		handler.DictTypeApiApp.GetInfo)
	dictType.POST("/export", satoken.CheckPermission("system:dict:export"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeExport), handler.DictTypeApiApp.Export)
	dictType.POST("", satoken.CheckPermission("system:dict:add"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictTypeApiApp.Add)
	dictType.PUT("", satoken.CheckPermission("system:dict:edit"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictTypeApiApp.Edit)
	// refreshCache 与 :dictIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh：对照 Java @SaCheckPermission("system:dict:remove")。
	dictType.DELETE("/refreshCache", satoken.CheckPermission("system:dict:remove"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeClean), handler.DictTypeApiApp.RefreshCache)
	dictType.DELETE("/:dictIds", satoken.CheckPermission("system:dict:remove"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeDelete), handler.DictTypeApiApp.Remove)
}

// InitRouter 构建并返回 system 进程的 gin 引擎(独立部署用)。
// 独立部署不带 /system 前缀，交给 nginx 代理时剥离。
func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.CORS())
	r.Use(middleware.TraceID())
	r.Use(middleware.RepeatableBody())
	r.Use(middleware.AccessLog())
	r.Use(middleware.XSS())
	r.Use(middleware.I18n())

	RegisterRoutes(r, "")

	return r
}
