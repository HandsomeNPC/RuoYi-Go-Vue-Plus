package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/apiencrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/constant"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/enum"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/permission"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/recover"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/oplog"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/loginhelper"
	systemconfig "ruoyi-go-vue-plus/internal/system/config"
	"ruoyi-go-vue-plus/internal/system/handler"
)

const clientLogTitle = "客户端管理"

const configLogTitle = "参数管理"

const deptLogTitle = "部门管理"

const dictDataLogTitle = "字典数据"

const dictTypeLogTitle = "字典类型"

const menuLogTitle = "菜单管理"

const noticeLogTitle = "通知公告"

const postLogTitle = "岗位管理"

const roleLogTitle = "角色管理"

const profileLogTitle = "个人信息"

const userLogTitle = "用户管理"

func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())
	enc := apiencrypt.APIEncryptWithConfig(systemconfig.Get().APIEncrypt)

	// 公开路由(免鉴权)：探针
	r.GET(prefix+"/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"module": "system", "message": "pong"})
	})
	protected := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	protected.Use(plugin.TokenInterceptor(), loginhelper.AuditContext())
	user := protected.Group("/user")
	user.GET("/getInfo", sagin.CheckLogin(), handler.UserApiApp.GetInfo)
	// 个人信息：不校验权限码，仅需登录——用户改自己的资料不该卡权限。
	// profile/avatar 端点暂未提供，前端上传留待 OSS 落地后再补。
	profile := user.Group("/profile")
	profile.GET("", sagin.CheckLogin(), handler.ProfileApiApp.Profile)
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	profile.PUT("", sagin.CheckLogin(),
		oplog.Log(profileLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ProfileApiApp.UpdateProfile)
	// updatePwd 须排在 apiencrypt 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	profile.PUT("/updatePwd", sagin.CheckLogin(), enc,
		oplog.Log(profileLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ProfileApiApp.UpdatePwd)

	// 用户管理：静态段（list/deptTree/optionselect/authRole/unlock/export/importData/importTemplate
	// /resetPwd/changeStatus）与同层 /:userId 共存，gin 静态段优先，无需调整注册顺序。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	user.GET("/list", permission.CheckPermission("system:user:list"), handler.UserApiApp.List)
	user.GET("/list/dept/:deptId", permission.CheckPermission("system:user:list"),
		handler.UserApiApp.ListByDept)
	user.GET("/deptTree", permission.CheckPermission("system:user:list"),
		handler.UserApiApp.DeptTree)
	user.GET("/optionselect", permission.CheckPermission("system:user:query"),
		handler.UserApiApp.OptionSelect)
	user.GET("/authRole/:userId", permission.CheckPermission("system:user:query"),
		handler.UserApiApp.AuthRole)
	user.GET("/unlock/:userId", permission.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeOther),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Unlock)
	user.POST("/export", permission.CheckPermission("system:user:export"),
		oplog.Log(userLogTitle, enum.BusinessTypeExport), handler.UserApiApp.Export)
	user.POST("/importData", permission.CheckPermission("system:user:import"),
		oplog.Log(userLogTitle, enum.BusinessTypeImport), handler.UserApiApp.ImportData)
	// importTemplate 不校验权限码，仅需登录：下载模板不该卡权限。
	user.POST("/importTemplate", sagin.CheckLogin(), handler.UserApiApp.ImportTemplate)
	// 路径用 "" 而非 "/"：后者会注册成 /user/。
	user.POST("", permission.CheckPermission("system:user:add"),
		oplog.Log(userLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Add)
	user.PUT("", permission.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.Edit)
	// resetPwd 须排在 apiencrypt 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	user.PUT("/resetPwd", permission.CheckPermission("system:user:resetPwd"), enc,
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.ResetPwd)
	user.PUT("/changeStatus", permission.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.ChangeStatus)
	// authRole 是授权操作，businessType=GRANT。
	user.PUT("/authRole", permission.CheckPermission("system:user:edit"),
		oplog.Log(userLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.UserApiApp.InsertAuthRole)
	// 根路径 "" 与 /:userId 复用 GetInfoByID。
	// 须注册在各静态段之后，静态段优先与 /:userId 共存。
	user.GET("", permission.CheckPermission("system:user:query"), handler.UserApiApp.GetInfoByID)
	user.GET("/:userId", permission.CheckPermission("system:user:query"),
		handler.UserApiApp.GetInfoByID)
	user.DELETE("/:userIds", permission.CheckPermission("system:user:remove"),
		oplog.Log(userLogTitle, enum.BusinessTypeDelete), handler.UserApiApp.Remove)

	menu := protected.Group("/menu")
	menu.GET("/getRouters", sagin.CheckLogin(), handler.MenuApiApp.GetRouters)
	// 多个中间件串联即 AND：各中间件都得过。注意这与单个 CheckPermission 内部多值的 OR 语义不同——那是"任一权限码命中放行"。
	menu.GET("/list", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:list"), handler.MenuApiApp.List)
	// treeselect 与 roleMenuTreeselect 只要 query 权限，不挂超管角色：
	// 角色授权界面要用它们，而分配角色的人未必是超管。
	menu.GET("/treeselect", permission.CheckPermission("system:menu:query"),
		handler.MenuApiApp.TreeSelect)
	menu.GET("/roleMenuTreeselect/:roleId", permission.CheckPermission("system:menu:query"),
		handler.MenuApiApp.RoleMenuTreeSelect)
	// getRouters/treeselect 等静态段与 :menuId 同层但静态段优先，无需调整注册顺序。
	menu.GET("/:menuId", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:query"), handler.MenuApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /menu/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	menu.POST("", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:add"),
		oplog.Log(menuLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.MenuApiApp.Add)
	menu.PUT("", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:edit"),
		oplog.Log(menuLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.MenuApiApp.Edit)
	// cascade/:menuIds 段更具体，与 /:menuId 可共存，静态段优先。
	menu.DELETE("/cascade/:menuIds", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:remove"),
		oplog.Log(menuLogTitle, enum.BusinessTypeDelete), handler.MenuApiApp.CascadeRemove)
	menu.DELETE("/:menuId", permission.CheckRole(constant.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:remove"),
		oplog.Log(menuLogTitle, enum.BusinessTypeDelete), handler.MenuApiApp.Remove)

	client := protected.Group("/client")
	client.GET("/list", permission.CheckPermission("system:client:list"), handler.ClientApiApp.List)
	client.GET("/:id", permission.CheckPermission("system:client:query"), handler.ClientApiApp.GetInfo)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	// 与 PUT ""/changeStatus 同理，路径更具体，须注册在 client.POST("") 之后。
	client.POST("/export", permission.CheckPermission("system:client:export"),
		oplog.Log(clientLogTitle, enum.BusinessTypeExport), handler.ClientApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /client/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	client.POST("", permission.CheckPermission("system:client:add"),
		oplog.Log(clientLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.ClientApiApp.Add)
	client.PUT("", permission.CheckPermission("system:client:edit"),
		oplog.Log(clientLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ClientApiApp.Edit)
	// changeStatus 不挂防重：它幂等——重复提交同一状态无副作用。须注册在 PUT "" 之后，路径更具体。
	client.PUT("/changeStatus", permission.CheckPermission("system:client:edit"),
		oplog.Log(clientLogTitle, enum.BusinessTypeUpdate),
		handler.ClientApiApp.ChangeStatus)
	client.DELETE("/:ids", permission.CheckPermission("system:client:remove"),
		oplog.Log(clientLogTitle, enum.BusinessTypeDelete),
		handler.ClientApiApp.Remove)

	config := protected.Group("/config")
	config.GET("/list", permission.CheckPermission("system:config:list"), handler.ConfigApiApp.List)
	// configKey 与 :configId 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端多处要读参数却未必有配置管理权限。
	config.GET("/configKey/:configKey", sagin.CheckLogin(), handler.ConfigApiApp.GetConfigKey)
	config.GET("/:configId", permission.CheckPermission("system:config:query"),
		handler.ConfigApiApp.GetInfo)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	config.POST("/export", permission.CheckPermission("system:config:export"),
		oplog.Log(configLogTitle, enum.BusinessTypeExport), handler.ConfigApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /config/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	config.POST("", permission.CheckPermission("system:config:add"),
		oplog.Log(configLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.Add)
	config.PUT("", permission.CheckPermission("system:config:edit"),
		oplog.Log(configLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.Edit)
	config.PUT("/updateByKey", permission.CheckPermission("system:config:edit"),
		oplog.Log(configLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.ConfigApiApp.UpdateByKey)
	// refreshCache 与 :configIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh。
	config.DELETE("/refreshCache", permission.CheckPermission("system:config:remove"),
		oplog.Log(configLogTitle, enum.BusinessTypeClean),
		handler.ConfigApiApp.RefreshCache)
	config.DELETE("/:configIds", permission.CheckPermission("system:config:remove"),
		oplog.Log(configLogTitle, enum.BusinessTypeDelete),
		handler.ConfigApiApp.Remove)

	dept := protected.Group("/dept")
	// optionselect、list/exclude/:deptId 与 /:deptId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	dept.GET("/list", permission.CheckPermission("system:dept:list"), handler.DeptApiApp.List)
	dept.GET("/list/exclude/:deptId", permission.CheckPermission("system:dept:list"),
		handler.DeptApiApp.ExcludeChild)
	dept.GET("/optionselect", permission.CheckPermission("system:dept:query"),
		handler.DeptApiApp.OptionSelect)
	dept.GET("/:deptId", permission.CheckPermission("system:dept:query"), handler.DeptApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /dept/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	dept.POST("", permission.CheckPermission("system:dept:add"),
		oplog.Log(deptLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DeptApiApp.Add)
	dept.PUT("", permission.CheckPermission("system:dept:edit"),
		oplog.Log(deptLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DeptApiApp.Edit)
	// 只支持单删：部门有父子关系，批量删无法保证删除顺序。
	dept.DELETE("/:deptId", permission.CheckPermission("system:dept:remove"),
		oplog.Log(deptLogTitle, enum.BusinessTypeDelete), handler.DeptApiApp.Remove)

	// 字典数据与字典类型是 /dict 下两个并列静态段，各自独立注册。
	// 权限码共用 system:dict:*，非 dict:data/dict:type 分设。
	dictData := protected.Group("/dict/data")
	dictData.GET("/list", permission.CheckPermission("system:dict:list"),
		handler.DictDataApiApp.List)
	// type/:dictType 与 :dictCode 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端 DictTag 到处渲染字典标签却未必有字典管理权限。
	dictData.GET("/type/:dictType", sagin.CheckLogin(), handler.DictDataApiApp.DictType)
	dictData.GET("/:dictCode", permission.CheckPermission("system:dict:query"),
		handler.DictDataApiApp.GetInfo)
	// 导出走 POST：前端 commonExport 以 form 表单提交筛选条件。
	dictData.POST("/export", permission.CheckPermission("system:dict:export"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeExport), handler.DictDataApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /dict/data/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	dictData.POST("", permission.CheckPermission("system:dict:add"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictDataApiApp.Add)
	dictData.PUT("", permission.CheckPermission("system:dict:edit"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictDataApiApp.Edit)
	dictData.DELETE("/:dictCodes", permission.CheckPermission("system:dict:remove"),
		oplog.Log(dictDataLogTitle, enum.BusinessTypeDelete), handler.DictDataApiApp.Remove)

	dictType := protected.Group("/dict/type")
	dictType.GET("/list", permission.CheckPermission("system:dict:list"),
		handler.DictTypeApiApp.List)
	// optionselect 与 :dictId 同层，静态段优先，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端选字典时未必有字典管理权限。
	dictType.GET("/optionselect", sagin.CheckLogin(), handler.DictTypeApiApp.OptionSelect)
	dictType.GET("/:dictId", permission.CheckPermission("system:dict:query"),
		handler.DictTypeApiApp.GetInfo)
	dictType.POST("/export", permission.CheckPermission("system:dict:export"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeExport), handler.DictTypeApiApp.Export)
	dictType.POST("", permission.CheckPermission("system:dict:add"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictTypeApiApp.Add)
	dictType.PUT("", permission.CheckPermission("system:dict:edit"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.DictTypeApiApp.Edit)
	// refreshCache 与 :dictIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh。
	dictType.DELETE("/refreshCache", permission.CheckPermission("system:dict:remove"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeClean), handler.DictTypeApiApp.RefreshCache)
	dictType.DELETE("/:dictIds", permission.CheckPermission("system:dict:remove"),
		oplog.Log(dictTypeLogTitle, enum.BusinessTypeDelete), handler.DictTypeApiApp.Remove)

	notice := protected.Group("/notice")
	notice.GET("/list", permission.CheckPermission("system:notice:list"),
		handler.NoticeApiApp.List)
	notice.GET("/:noticeId", permission.CheckPermission("system:notice:query"),
		handler.NoticeApiApp.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /notice/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	notice.POST("", permission.CheckPermission("system:notice:add"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.NoticeApiApp.Add)
	notice.PUT("", permission.CheckPermission("system:notice:edit"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.NoticeApiApp.Edit)
	// notice 无导出接口，故不注册 /export。
	notice.DELETE("/:noticeIds", permission.CheckPermission("system:notice:remove"),
		oplog.Log(noticeLogTitle, enum.BusinessTypeDelete), handler.NoticeApiApp.Remove)

	post := protected.Group("/post")
	post.GET("/list", permission.CheckPermission("system:post:list"), handler.PostApiApp.List)
	// optionselect、deptTree 与 :postId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	post.GET("/optionselect", permission.CheckPermission("system:post:query"),
		handler.PostApiApp.OptionSelect)
	post.GET("/deptTree", permission.CheckPermission("system:post:list"),
		handler.PostApiApp.DeptTree)
	post.GET("/:postId", permission.CheckPermission("system:post:query"),
		handler.PostApiApp.GetInfo)
	// 导出走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件。
	post.POST("/export", permission.CheckPermission("system:post:export"),
		oplog.Log(postLogTitle, enum.BusinessTypeExport), handler.PostApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /post/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	post.POST("", permission.CheckPermission("system:post:add"),
		oplog.Log(postLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.PostApiApp.Add)
	post.PUT("", permission.CheckPermission("system:post:edit"),
		oplog.Log(postLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.PostApiApp.Edit)
	post.DELETE("/:postIds", permission.CheckPermission("system:post:remove"),
		oplog.Log(postLogTitle, enum.BusinessTypeDelete), handler.PostApiApp.Remove)

	// social 不校验权限码，仅需登录：用户查自己的社会化绑定关系不该卡权限。
	social := protected.Group("/social")
	social.GET("/list", sagin.CheckLogin(), handler.SocialApiApp.List)

	role := protected.Group("/role")
	role.GET("/list", permission.CheckPermission("system:role:list"), handler.RoleApiApp.List)
	// optionselect、authUser/*、deptTree/:roleId 与 :roleId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	role.GET("/optionselect", permission.CheckPermission("system:role:query"),
		handler.RoleApiApp.OptionSelect)
	role.GET("/authUser/allocatedList", permission.CheckPermission("system:role:list"),
		handler.RoleApiApp.AllocatedList)
	role.GET("/authUser/unallocatedList", permission.CheckPermission("system:role:list"),
		handler.RoleApiApp.UnallocatedList)
	// deptTree/:roleId 取角色部门勾选 + 部门下拉树。
	role.GET("/deptTree/:roleId", permission.CheckPermission("system:role:list"),
		handler.RoleApiApp.DeptTreeSelect)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	role.POST("/export", permission.CheckPermission("system:role:export"),
		oplog.Log(roleLogTitle, enum.BusinessTypeExport), handler.RoleApiApp.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /role/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	role.POST("", permission.CheckPermission("system:role:add"),
		oplog.Log(roleLogTitle, enum.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.Add)
	role.PUT("", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.Edit)
	// permission、changeStatus 路径更具体，须注册在 PUT "" 之后。
	role.PUT("/permission", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.EditPermission)
	role.PUT("/changeStatus", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.ChangeStatus)
	// authUser/cancel、cancelAll、selectAll 是授权操作，businessType=GRANT。
	role.PUT("/authUser/cancel", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.CancelAuthUser)
	role.PUT("/authUser/cancelAll", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.CancelAuthUserAll)
	role.PUT("/authUser/selectAll", permission.CheckPermission("system:role:edit"),
		oplog.Log(roleLogTitle, enum.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), handler.RoleApiApp.SelectAuthUserAll)
	role.GET("/:roleId", permission.CheckPermission("system:role:query"),
		handler.RoleApiApp.GetInfo)
	role.DELETE("/:roleIds", permission.CheckPermission("system:role:remove"),
		oplog.Log(roleLogTitle, enum.BusinessTypeDelete), handler.RoleApiApp.Remove)
}

// InitRouter 构建并返回 system 进程的 gin 引擎(独立部署用)。
// 独立部署不带 /system 前缀，交给 nginx 代理时剥离。
//
// /resource/* 的路由（消息盒子、推送端点）归 internal/resource——与 OSS 一起由 resource 进程承载。
func InitRouter() *gin.Engine {
	r := gin.New()
	cfg := systemconfig.Get()

	r.Use(recover.Recover())
	r.Use(cors.CORSWithConfig(cfg.CORS))
	r.Use(trace.TraceIDWithConfig(cfg.TraceID))
	r.Use(repeatablebody.RepeatableBodyWithConfig(cfg.RepeatableBody))
	r.Use(accesslog.AccessLogWithConfig(cfg.AccessLog))
	r.Use(xss.XSSWithConfig(cfg.XSS))

	RegisterRoutes(r, "")

	return r
}
