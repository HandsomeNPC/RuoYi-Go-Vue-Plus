package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"ruoyi-go-vue-plus/internal/system/handler"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/enum"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/push"
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

// noticeLogTitle 通知公告的操作日志模块名，对照 Java @Log(title = "通知公告")。
const noticeLogTitle = "通知公告"

// postLogTitle 岗位管理的操作日志模块名，对照 Java @Log(title = "岗位管理")。
const postLogTitle = "岗位管理"

// roleLogTitle 角色管理的操作日志模块名，对照 Java @Log(title = "角色管理")。
const roleLogTitle = "角色管理"

// profileLogTitle 个人信息的操作日志模块名，对照 Java @Log(title = "个人信息")。
const profileLogTitle = "个人信息"

// userLogTitle 用户管理的操作日志模块名，对照 Java @Log(title = "用户管理")。
const userLogTitle = "用户管理"

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
	// 个人信息：与 Java 一致不校验权限码，仅需登录——用户改自己的资料不该卡权限。
	// profile/avatar 端点 Java 的 SysProfileController 未提供，前端的上传留待 OSS 落地后再补。
	profile := user.Group("/profile")
	profile.GET("", sagin.CheckLogin(), handler.ProfileApiApp.Profile)
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	profile.PUT("", sagin.CheckLogin(),
		oplog.Log(profileLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ProfileApiApp.UpdateProfile)
	// updatePwd 须排在 encrypt.ApiEncrypt() 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	profile.PUT("/updatePwd", sagin.CheckLogin(), encrypt.ApiEncrypt(),
		oplog.Log(profileLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ProfileApiApp.UpdatePwd)

	// 用户管理：静态段（list/deptTree/optionselect/authRole/unlock/export/importData/importTemplate
	// /resetPwd/changeStatus）与同层 /:userId 共存，gin 静态段优先，无需调整注册顺序。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	user.GET("/list", satoken.CheckPermission("system:user:list"), handler.UserApiApp.List)
	user.GET("/list/dept/:deptId", satoken.CheckPermission("system:user:list"),
		handler.UserApiApp.ListByDept)
	user.GET("/deptTree", satoken.CheckPermission("system:user:list"),
		handler.UserApiApp.DeptTree)
	user.GET("/optionselect", satoken.CheckPermission("system:user:query"),
		handler.UserApiApp.OptionSelect)
	user.GET("/authRole/:userId", satoken.CheckPermission("system:user:query"),
		handler.UserApiApp.AuthRole)
	user.GET("/unlock/:userId", satoken.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeOther),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Unlock)
	user.POST("/export", satoken.CheckPermission("system:user:export"),
		oplog.Log(userLogTitle, enum.BusinessTypeExport), handler.UserApiApp.Export)
	user.POST("/importData", satoken.CheckPermission("system:user:import"),
		oplog.Log(userLogTitle, enum.BusinessTypeImport), handler.UserApiApp.ImportData)
	// importTemplate 与 Java 一致不校验权限码，仅需登录：下载模板不该卡权限。
	user.POST("/importTemplate", sagin.CheckLogin(), handler.UserApiApp.ImportTemplate)
	// 路径用 "" 而非 "/"：后者会注册成 /user/。
	user.POST("", satoken.CheckPermission("system:user:add"),
		oplog.Log(userLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Add)
	user.PUT("", satoken.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Edit)
	// resetPwd 须排在 encrypt.ApiEncrypt() 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	user.PUT("/resetPwd", satoken.CheckPermission("system:user:resetPwd"), encrypt.ApiEncrypt(),
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.ResetPwd)
	user.PUT("/changeStatus", satoken.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.ChangeStatus)
	// authRole 是授权操作，businessType=GRANT。
	user.PUT("/authRole", satoken.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.InsertAuthRole)
	// 根路径 "" 与 /:userId 复用 GetInfoByID：Java 的 @GetMapping({"/","/{userId}"}) 同一方法。
	// 须注册在各静态段之后，静态段优先与 /:userId 共存。
	user.GET("", satoken.CheckPermission("system:user:query"), handler.UserApiApp.GetInfoByID)
	user.GET("/:userId", satoken.CheckPermission("system:user:query"),
		handler.UserApiApp.GetInfoByID)
	user.DELETE("/:userIds", satoken.CheckPermission("system:user:remove"),
		oplog.Log(userLogTitle, enum.BusinessTypeDelete), handler.UserApiApp.Remove)

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

	notice := protected.Group("/notice")
	notice.GET("/list", satoken.CheckPermission("system:notice:list"),
		handler.NoticeApiApp.List)
	notice.GET("/:noticeId", satoken.CheckPermission("system:notice:query"),
		handler.NoticeApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /notice/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	notice.POST("", satoken.CheckPermission("system:notice:add"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.NoticeApiApp.Add)
	notice.PUT("", satoken.CheckPermission("system:notice:edit"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.NoticeApiApp.Edit)
	// Java 侧 notice 没有导出接口，故不注册 /export。
	notice.DELETE("/:noticeIds", satoken.CheckPermission("system:notice:remove"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeDelete), handler.NoticeApiApp.Remove)

	post := protected.Group("/post")
	post.GET("/list", satoken.CheckPermission("system:post:list"), handler.PostApiApp.List)
	// optionselect、deptTree 与 :postId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	post.GET("/optionselect", satoken.CheckPermission("system:post:query"),
		handler.PostApiApp.OptionSelect)
	post.GET("/deptTree", satoken.CheckPermission("system:post:list"),
		handler.PostApiApp.DeptTree)
	post.GET("/:postId", satoken.CheckPermission("system:post:query"),
		handler.PostApiApp.GetInfo)
	// 导出走 POST 与 Java 一致：前端 commonExport 以 form 表单 POST 提交筛选条件。
	post.POST("/export", satoken.CheckPermission("system:post:export"),
		oplog.Log(postLogTitle, enum.BusinessTypeExport), handler.PostApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /post/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	post.POST("", satoken.CheckPermission("system:post:add"),
		oplog.Log(postLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.PostApiApp.Add)
	post.PUT("", satoken.CheckPermission("system:post:edit"),
		oplog.Log(postLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.PostApiApp.Edit)
	post.DELETE("/:postIds", satoken.CheckPermission("system:post:remove"),
		oplog.Log(postLogTitle, enum.BusinessTypeDelete), handler.PostApiApp.Remove)

	// social 与 Java 一致不校验权限码，仅需登录：用户查自己的社会化绑定关系不该卡权限。
	social := protected.Group("/social")
	social.GET("/list", sagin.CheckLogin(), handler.SocialApiApp.List)

	// roleLogTitle 角色管理的操作日志模块名，对照 Java @Log(title = "角色管理")。
	role := protected.Group("/role")
	role.GET("/list", satoken.CheckPermission("system:role:list"), handler.RoleApiApp.List)
	// optionselect、authUser/*、deptTree/:roleId 与 :roleId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	role.GET("/optionselect", satoken.CheckPermission("system:role:query"),
		handler.RoleApiApp.OptionSelect)
	role.GET("/authUser/allocatedList", satoken.CheckPermission("system:role:list"),
		handler.RoleApiApp.AllocatedList)
	role.GET("/authUser/unallocatedList", satoken.CheckPermission("system:role:list"),
		handler.RoleApiApp.UnallocatedList)
	// deptTree/:roleId 取角色部门勾选 + 部门下拉树，对照 Java roleDeptTreeselect。
	role.GET("/deptTree/:roleId", satoken.CheckPermission("system:role:list"),
		handler.RoleApiApp.DeptTreeSelect)
	// 导出走 POST 与 Java 一致：前端以 form 表单 POST 提交筛选条件。
	role.POST("/export", satoken.CheckPermission("system:role:export"),
		oplog.Log(roleLogTitle, enum.BusinessTypeExport), handler.RoleApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /role/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行，与 Java 侧
	// RepeatSubmitAspect 抛异常后 LogAspect 记一条失败日志一致。
	role.POST("", satoken.CheckPermission("system:role:add"),
		oplog.Log(roleLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.Add)
	role.PUT("", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.Edit)
	// permission、changeStatus 路径更具体，须注册在 PUT "" 之后。
	role.PUT("/permission", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.EditPermission)
	role.PUT("/changeStatus", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.ChangeStatus)
	// authUser/cancel、cancelAll、selectAll 是授权操作，businessType=GRANT。
	role.PUT("/authUser/cancel", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.CancelAuthUser)
	role.PUT("/authUser/cancelAll", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.CancelAuthUserAll)
	role.PUT("/authUser/selectAll", satoken.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.SelectAuthUserAll)
	role.GET("/:roleId", satoken.CheckPermission("system:role:query"),
		handler.RoleApiApp.GetInfo)
	role.DELETE("/:roleIds", satoken.CheckPermission("system:role:remove"),
		oplog.Log(roleLogTitle, enum.BusinessTypeDelete), handler.RoleApiApp.Remove)
}

// RegisterResourceRoutes 注册消息盒子与推送连接端点。
//
// 单独成函数而非并入 RegisterRoutes：这些路由在 Java 侧挂在 /resource/* 下，
// 不带 /system 前缀。standalone 部署要把它们注册在 /system 之外，
// 而 modular 部署下 system 进程本身无前缀、由 nginx 另配 location 转发，
// 两种部署对前缀的要求不同，只能由调用方各自决定。
func RegisterResourceRoutes(r *gin.Engine) {
	plugin := sagin.NewPlugin(satoken.Manager())

	resource := r.Group("/resource")
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	resource.Use(plugin.TokenInterceptor(), loginhelper.AuditContext())

	// 与 Java 一致不校验权限码，仅需登录：消息盒子只返回当前用户自己的消息。
	resource.GET("/message/box", sagin.CheckLogin(), handler.MessageApiApp.GetBox)

	// 推送端点按 push.transport 决定走 SSE 还是 WebSocket，路径取配置值。
	// 未启用推送时不注册，避免前端连上一个只会报错的端点。
	cfg := config.Get().Push
	if !cfg.Enabled {
		return
	}
	// NormalizeQueryToken 须排在 TokenInterceptor 之前：EventSource/WebSocket
	// 不能自定义请求头，token 只能走 query，形如 ?Authorization=Bearer xxx，
	// 而 sa-token-go 的 query 分支不剥 Bearer 前缀，不规范化会一律 401。
	r.GET(cfg.Path, push.NormalizeQueryToken(), plugin.TokenInterceptor(),
		sagin.CheckLogin(), push.Handler())
	// close 对齐 Java SseController.close：前端主动断开时清掉服务端会话，
	// 不必等心跳超时才回收。
	r.GET(cfg.Path+"/close", push.NormalizeQueryToken(), plugin.TokenInterceptor(),
		sagin.CheckLogin(), push.CloseHandler())
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
	// /resource/* 本就不带模块前缀，与业务路由同层注册即可；
	// nginx 侧需另配 location /resource/ 转到本进程。
	RegisterResourceRoutes(r)

	return r
}
