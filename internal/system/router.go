package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/apiencrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatsubmit"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/audit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/checklogin"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/notfound"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/operlog"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/permission"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/recover"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	systemconfig "ruoyi-go-vue-plus/internal/system/config"
	syscons "ruoyi-go-vue-plus/internal/system/constant"
	clienthandler "ruoyi-go-vue-plus/internal/system/handler/client"
	confighandler "ruoyi-go-vue-plus/internal/system/handler/config"
	depthandler "ruoyi-go-vue-plus/internal/system/handler/dept"
	dictdatahandler "ruoyi-go-vue-plus/internal/system/handler/dictdata"
	dicttypehandler "ruoyi-go-vue-plus/internal/system/handler/dicttype"
	menuhandler "ruoyi-go-vue-plus/internal/system/handler/menu"
	noticehandler "ruoyi-go-vue-plus/internal/system/handler/notice"
	posthandler "ruoyi-go-vue-plus/internal/system/handler/post"
	profilehandler "ruoyi-go-vue-plus/internal/system/handler/profile"
	rolehandler "ruoyi-go-vue-plus/internal/system/handler/role"
	socialhandler "ruoyi-go-vue-plus/internal/system/handler/social"
	userhandler "ruoyi-go-vue-plus/internal/system/handler/user"
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
		c.JSON(http.StatusOK, response.Ok(gin.H{"module": "system", "message": "pong"}))
	})
	protected := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后：它取的登录态依赖后者解析出的 token。
	protected.Use(plugin.TokenInterceptor(), audit.AuditContext())
	user := protected.Group("/user")
	user.GET("/getInfo", checklogin.CheckLogin(), userhandler.GetInfo)
	// 个人信息：不校验权限码，仅需登录——用户改自己的资料不该卡权限。
	// profile/avatar 端点暂未提供，前端上传留待 OSS 落地后再补。
	profile := user.Group("/profile")
	profile.GET("", checklogin.CheckLogin(), profilehandler.Profile)
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	profile.PUT("", checklogin.CheckLogin(),
		operlog.Log(profileLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), profilehandler.UpdateProfile)
	// updatePwd 须排在 apiencrypt 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	profile.PUT("/updatePwd", checklogin.CheckLogin(), enc,
		operlog.Log(profileLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), profilehandler.UpdatePwd)

	// 用户管理：静态段（list/deptTree/optionselect/authRole/unlock/export/importData/importTemplate
	// /resetPwd/changeStatus）与同层 /:userId 共存，gin 静态段优先，无需调整注册顺序。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	user.GET("/list", permission.CheckPermission("system:user:list"), userhandler.List)
	user.GET("/list/dept/:deptId", permission.CheckPermission("system:user:list"),
		userhandler.ListByDept)
	user.GET("/deptTree", permission.CheckPermission("system:user:list"),
		userhandler.DeptTree)
	user.GET("/optionselect", permission.CheckPermission("system:user:query"),
		userhandler.OptionSelect)
	user.GET("/authRole/:userId", permission.CheckPermission("system:user:query"),
		userhandler.AuthRole)
	user.GET("/unlock/:userId", permission.CheckPermission("system:user:edit"),
		operlog.Log(userLogTitle, operlog.BusinessTypeOther),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.Unlock)
	user.POST("/export", permission.CheckPermission("system:user:export"),
		operlog.Log(userLogTitle, operlog.BusinessTypeExport), userhandler.Export)
	user.POST("/importData", permission.CheckPermission("system:user:import"),
		operlog.Log(userLogTitle, operlog.BusinessTypeImport), userhandler.ImportData)
	// importTemplate 不校验权限码，仅需登录：下载模板不该卡权限。
	user.POST("/importTemplate", checklogin.CheckLogin(), userhandler.ImportTemplate)
	// 路径用 "" 而非 "/"：后者会注册成 /user/。
	user.POST("", permission.CheckPermission("system:user:add"),
		operlog.Log(userLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.Add)
	user.PUT("", permission.CheckPermission("system:user:edit"),
		operlog.Log(userLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.Edit)
	// resetPwd 须排在 apiencrypt 之后：指纹要用解密后的明文，否则密文每次
	// 随机密钥、同样入参算出不同指纹，防重直接失效。故顺序为 鉴权→解密→日志→防重→handler。
	user.PUT("/resetPwd", permission.CheckPermission("system:user:resetPwd"), enc,
		operlog.Log(userLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.ResetPwd)
	user.PUT("/changeStatus", permission.CheckPermission("system:user:edit"),
		operlog.Log(userLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.ChangeStatus)
	// authRole 是授权操作，businessType=GRANT。
	user.PUT("/authRole", permission.CheckPermission("system:user:edit"),
		operlog.Log(userLogTitle, operlog.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), userhandler.InsertAuthRole)
	// 根路径 "" 与 /:userId 复用 GetInfoByID。
	// 须注册在各静态段之后，静态段优先与 /:userId 共存。
	user.GET("", permission.CheckPermission("system:user:query"), userhandler.GetInfoByID)
	user.GET("/:userId", permission.CheckPermission("system:user:query"),
		userhandler.GetInfoByID)
	user.DELETE("/:userIds", permission.CheckPermission("system:user:remove"),
		operlog.Log(userLogTitle, operlog.BusinessTypeDelete), userhandler.Remove)

	menu := protected.Group("/menu")
	menu.GET("/getRouters", checklogin.CheckLogin(), menuhandler.GetRouters)
	// 多个中间件串联即 AND：各中间件都得过。注意这与单个 CheckPermission 内部多值的 OR 语义不同——那是"任一权限码命中放行"。
	menu.GET("/list", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:list"), menuhandler.List)
	// treeselect 与 roleMenuTreeselect 只要 query 权限，不挂超管角色：
	// 角色授权界面要用它们，而分配角色的人未必是超管。
	menu.GET("/treeselect", permission.CheckPermission("system:menu:query"),
		menuhandler.TreeSelect)
	menu.GET("/roleMenuTreeselect/:roleId", permission.CheckPermission("system:menu:query"),
		menuhandler.RoleMenuTreeSelect)
	// getRouters/treeselect 等静态段与 :menuId 同层但静态段优先，无需调整注册顺序。
	menu.GET("/:menuId", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:query"), menuhandler.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /menu/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	menu.POST("", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:add"),
		operlog.Log(menuLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), menuhandler.Add)
	menu.PUT("", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:edit"),
		operlog.Log(menuLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), menuhandler.Edit)
	// cascade/:menuIds 段更具体，与 /:menuId 可共存，静态段优先。
	menu.DELETE("/cascade/:menuIds", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:remove"),
		operlog.Log(menuLogTitle, operlog.BusinessTypeDelete), menuhandler.CascadeRemove)
	menu.DELETE("/:menuId", permission.CheckRole(syscons.SuperAdminRoleKey),
		permission.CheckPermission("system:menu:remove"),
		operlog.Log(menuLogTitle, operlog.BusinessTypeDelete), menuhandler.Remove)

	client := protected.Group("/client")
	client.GET("/list", permission.CheckPermission("system:client:list"), clienthandler.List)
	client.GET("/:id", permission.CheckPermission("system:client:query"), clienthandler.GetInfo)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	// 与 PUT ""/changeStatus 同理，路径更具体，须注册在 client.POST("") 之后。
	client.POST("/export", permission.CheckPermission("system:client:export"),
		operlog.Log(clientLogTitle, operlog.BusinessTypeExport), clienthandler.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /client/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	client.POST("", permission.CheckPermission("system:client:add"),
		operlog.Log(clientLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), clienthandler.Add)
	client.PUT("", permission.CheckPermission("system:client:edit"),
		operlog.Log(clientLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), clienthandler.Edit)
	// changeStatus 不挂防重：它幂等——重复提交同一状态无副作用。须注册在 PUT "" 之后，路径更具体。
	client.PUT("/changeStatus", permission.CheckPermission("system:client:edit"),
		operlog.Log(clientLogTitle, operlog.BusinessTypeUpdate),
		clienthandler.ChangeStatus)
	client.DELETE("/:ids", permission.CheckPermission("system:client:remove"),
		operlog.Log(clientLogTitle, operlog.BusinessTypeDelete),
		clienthandler.Remove)

	config := protected.Group("/config")
	config.GET("/list", permission.CheckPermission("system:config:list"), confighandler.List)
	// configKey 与 :configId 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端多处要读参数却未必有配置管理权限。
	config.GET("/configKey/:configKey", checklogin.CheckLogin(), confighandler.GetConfigKey)
	config.GET("/:configId", permission.CheckPermission("system:config:query"),
		confighandler.GetInfo)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	config.POST("/export", permission.CheckPermission("system:config:export"),
		operlog.Log(configLogTitle, operlog.BusinessTypeExport), confighandler.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /config/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	config.POST("", permission.CheckPermission("system:config:add"),
		operlog.Log(configLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), confighandler.Add)
	config.PUT("", permission.CheckPermission("system:config:edit"),
		operlog.Log(configLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), confighandler.Edit)
	config.PUT("/updateByKey", permission.CheckPermission("system:config:edit"),
		operlog.Log(configLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), confighandler.UpdateByKey)
	// refreshCache 与 :configIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh。
	config.DELETE("/refreshCache", permission.CheckPermission("system:config:remove"),
		operlog.Log(configLogTitle, operlog.BusinessTypeClean),
		confighandler.RefreshCache)
	config.DELETE("/:configIds", permission.CheckPermission("system:config:remove"),
		operlog.Log(configLogTitle, operlog.BusinessTypeDelete),
		confighandler.Remove)

	dept := protected.Group("/dept")
	// optionselect、list/exclude/:deptId 与 /:deptId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	dept.GET("/list", permission.CheckPermission("system:dept:list"), depthandler.List)
	dept.GET("/list/exclude/:deptId", permission.CheckPermission("system:dept:list"),
		depthandler.ExcludeChild)
	dept.GET("/optionselect", permission.CheckPermission("system:dept:query"),
		depthandler.OptionSelect)
	dept.GET("/:deptId", permission.CheckPermission("system:dept:query"), depthandler.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /dept/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	dept.POST("", permission.CheckPermission("system:dept:add"),
		operlog.Log(deptLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), depthandler.Add)
	dept.PUT("", permission.CheckPermission("system:dept:edit"),
		operlog.Log(deptLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), depthandler.Edit)
	// 只支持单删：部门有父子关系，批量删无法保证删除顺序。
	dept.DELETE("/:deptId", permission.CheckPermission("system:dept:remove"),
		operlog.Log(deptLogTitle, operlog.BusinessTypeDelete), depthandler.Remove)

	// 字典数据与字典类型是 /dict 下两个并列静态段，各自独立注册。
	// 权限码共用 system:dict:*，非 dict:data/dict:type 分设。
	dictData := protected.Group("/dict/data")
	dictData.GET("/list", permission.CheckPermission("system:dict:list"),
		dictdatahandler.List)
	// type/:dictType 与 :dictCode 同层但静态段优先，gin 能区分二者，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端 DictTag 到处渲染字典标签却未必有字典管理权限。
	dictData.GET("/type/:dictType", checklogin.CheckLogin(), dictdatahandler.DictType)
	dictData.GET("/:dictCode", permission.CheckPermission("system:dict:query"),
		dictdatahandler.GetInfo)
	// 导出走 POST：前端 commonExport 以 form 表单提交筛选条件。
	dictData.POST("/export", permission.CheckPermission("system:dict:export"),
		operlog.Log(dictDataLogTitle, operlog.BusinessTypeExport), dictdatahandler.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /dict/data/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	dictData.POST("", permission.CheckPermission("system:dict:add"),
		operlog.Log(dictDataLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), dictdatahandler.Add)
	dictData.PUT("", permission.CheckPermission("system:dict:edit"),
		operlog.Log(dictDataLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), dictdatahandler.Edit)
	dictData.DELETE("/:dictCodes", permission.CheckPermission("system:dict:remove"),
		operlog.Log(dictDataLogTitle, operlog.BusinessTypeDelete), dictdatahandler.Remove)

	dictType := protected.Group("/dict/type")
	dictType.GET("/list", permission.CheckPermission("system:dict:list"),
		dicttypehandler.List)
	// optionselect 与 :dictId 同层，静态段优先，无需调整注册顺序。
	// 不校验权限码，仅需登录：前端选字典时未必有字典管理权限。
	dictType.GET("/optionselect", checklogin.CheckLogin(), dicttypehandler.OptionSelect)
	dictType.GET("/:dictId", permission.CheckPermission("system:dict:query"),
		dicttypehandler.GetInfo)
	dictType.POST("/export", permission.CheckPermission("system:dict:export"),
		operlog.Log(dictTypeLogTitle, operlog.BusinessTypeExport), dicttypehandler.Export)
	dictType.POST("", permission.CheckPermission("system:dict:add"),
		operlog.Log(dictTypeLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), dicttypehandler.Add)
	dictType.PUT("", permission.CheckPermission("system:dict:edit"),
		operlog.Log(dictTypeLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), dicttypehandler.Edit)
	// refreshCache 与 :dictIds 同层，静态段优先，无需调整注册顺序。
	// 权限码用 remove 而非独立的 refresh。
	dictType.DELETE("/refreshCache", permission.CheckPermission("system:dict:remove"),
		operlog.Log(dictTypeLogTitle, operlog.BusinessTypeClean), dicttypehandler.RefreshCache)
	dictType.DELETE("/:dictIds", permission.CheckPermission("system:dict:remove"),
		operlog.Log(dictTypeLogTitle, operlog.BusinessTypeDelete), dicttypehandler.Remove)

	notice := protected.Group("/notice")
	notice.GET("/list", permission.CheckPermission("system:notice:list"),
		noticehandler.List)
	notice.GET("/:noticeId", permission.CheckPermission("system:notice:query"),
		noticehandler.GetInfo)
	// 路径用 "" 而非 "/"：后者会注册成 /notice/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	notice.POST("", permission.CheckPermission("system:notice:add"),
		operlog.Log(noticeLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), noticehandler.Add)
	notice.PUT("", permission.CheckPermission("system:notice:edit"),
		operlog.Log(noticeLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), noticehandler.Edit)
	// notice 无导出接口，故不注册 /export。
	notice.DELETE("/:noticeIds", permission.CheckPermission("system:notice:remove"),
		operlog.Log(noticeLogTitle, operlog.BusinessTypeDelete), noticehandler.Remove)

	post := protected.Group("/post")
	post.GET("/list", permission.CheckPermission("system:post:list"), posthandler.List)
	// optionselect、deptTree 与 :postId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	post.GET("/optionselect", permission.CheckPermission("system:post:query"),
		posthandler.OptionSelect)
	post.GET("/deptTree", permission.CheckPermission("system:post:list"),
		posthandler.DeptTree)
	post.GET("/:postId", permission.CheckPermission("system:post:query"),
		posthandler.GetInfo)
	// 导出走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件。
	post.POST("/export", permission.CheckPermission("system:post:export"),
		operlog.Log(postLogTitle, operlog.BusinessTypeExport), posthandler.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /post/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	post.POST("", permission.CheckPermission("system:post:add"),
		operlog.Log(postLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), posthandler.Add)
	post.PUT("", permission.CheckPermission("system:post:edit"),
		operlog.Log(postLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), posthandler.Edit)
	post.DELETE("/:postIds", permission.CheckPermission("system:post:remove"),
		operlog.Log(postLogTitle, operlog.BusinessTypeDelete), posthandler.Remove)

	// social 不校验权限码，仅需登录：用户查自己的社会化绑定关系不该卡权限。
	social := protected.Group("/social")
	social.GET("/list", checklogin.CheckLogin(), socialhandler.List)

	role := protected.Group("/role")
	role.GET("/list", permission.CheckPermission("system:role:list"), rolehandler.List)
	// optionselect、authUser/*、deptTree/:roleId 与 :roleId 同层但前两者段更具体，
	// gin 静态段优先，无需调整注册顺序。
	role.GET("/optionselect", permission.CheckPermission("system:role:query"),
		rolehandler.OptionSelect)
	role.GET("/authUser/allocatedList", permission.CheckPermission("system:role:list"),
		rolehandler.AllocatedList)
	role.GET("/authUser/unallocatedList", permission.CheckPermission("system:role:list"),
		rolehandler.UnallocatedList)
	// deptTree/:roleId 取角色部门勾选 + 部门下拉树。
	role.GET("/deptTree/:roleId", permission.CheckPermission("system:role:list"),
		rolehandler.DeptTreeSelect)
	// 导出走 POST：前端以 form 表单 POST 提交筛选条件。
	role.POST("/export", permission.CheckPermission("system:role:export"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeExport), rolehandler.Export)
	// 路径用 "" 而非 "/"：后者会注册成 /role/。
	// 鉴权排在防重之前，未授权请求不该白占一个防重锁。
	// 操作日志排在防重之前：被防重挡掉的请求 handler 没执行。
	role.POST("", permission.CheckPermission("system:role:add"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeInsert),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.Add)
	role.PUT("", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.Edit)
	// permission、changeStatus 路径更具体，须注册在 PUT "" 之后。
	role.PUT("/permission", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.EditPermission)
	role.PUT("/changeStatus", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeUpdate),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.ChangeStatus)
	// authUser/cancel、cancelAll、selectAll 是授权操作，businessType=GRANT。
	role.PUT("/authUser/cancel", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.CancelAuthUser)
	role.PUT("/authUser/cancelAll", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.CancelAuthUserAll)
	role.PUT("/authUser/selectAll", permission.CheckPermission("system:role:edit"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeGrant),
		repeatsubmit.RepeatSubmit(0, ""), rolehandler.SelectAuthUserAll)
	role.GET("/:roleId", permission.CheckPermission("system:role:query"),
		rolehandler.GetInfo)
	role.DELETE("/:roleIds", permission.CheckPermission("system:role:remove"),
		operlog.Log(roleLogTitle, operlog.BusinessTypeDelete), rolehandler.Remove)
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
	notfound.Register(r)

	return r
}
