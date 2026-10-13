// Package constant system 模块专属常量。
//
// 这些值只有 system 自己用（跨模块共享的那些留在各自生产者模块），
// 散落在 model/repository/service 多个子包，故在模块层收口一处。
package constant

import "time"

// 通用状态与是否标记。落 sys_* 表的 status / 各类 Y-N 列。
const (
	StatusNormal  = "0"
	StatusDisable = "1"
	Yes           = "Y"
	No            = "N"
)

// 菜单类型与前端组件名。
const (
	MenuTypeDir    = "M"
	MenuTypeMenu   = "C"
	MenuTypeButton = "F"

	ComponentLayout     = "Layout"
	ComponentParentView = "ParentView"
	ComponentInnerLink  = "InnerLink"
)

// 超管与部门根节点。
const (
	SuperAdminUserID  int64 = 1761100000000000001
	SuperAdminRoleID  int64 = 1761300000000000001
	SuperAdminRoleKey       = "superadmin"

	RootDeptAncestors       = "0"
	DefaultDeptID     int64 = 1761000000000000100
	// TopParentID 树形结构的根节点父ID。
	TopParentID int64 = 0
)

// 登录日志状态归类用的成功/失败标记，落 sys_login_info.status。
const (
	LogStatusSuccess = "0"
	LogStatusFail    = "1"
)

// URL 前缀判定，用于菜单外链识别。
const (
	WWW   = "www."
	HTTP  = "http://"
	HTTPS = "https://"
)

// 消息盒子分类。
// 落库进 sys_message.category，前端按它分三个 tab 展示。
const (
	// MessageCategorySystem 系统消息。
	MessageCategorySystem = "system"
	// MessageCategoryNotice 通知公告。
	MessageCategoryNotice = "notice"
	// MessageCategoryWorkflow 工作流消息。
	MessageCategoryWorkflow = "workflow"
)

// MessageGlobalUserIDs 全局广播的接收人标识，落 sys_message.send_user_ids。
// 全部用户都能查到。
const MessageGlobalUserIDs = "0"

// 缓存组名。
const (
	CacheSysConfig       = "sys_config"
	CacheSysDict         = "sys_dict"
	CacheSysDictType     = "sys_dict_type"
	CacheSysClient       = "sys_client"
	CacheSysUserName     = "sys_user_name"
	CacheSysNickname     = "sys_nickname"
	CacheSysDept         = "sys_dept"
	CacheSysRoleCustom   = "sys_role_custom"
	CacheSysDeptAndChild = "sys_dept_and_child"
)

// 缓存组过期时间。
const (
	// CacheTTLSysConfig 参数配置不过期（key 不带 #ttl 后缀）。
	// 参数值极少变动且写路径都带缓存维护（新增/修改回写、删除失效），无需靠 TTL 兜底。
	CacheTTLSysConfig = 0
	// CacheTTLSysDict/CacheTTLSysDictType 字典不过期（key 不带 #ttl 后缀）。
	// 字典是全站高频读的静态数据，且写路径都带
	// 缓存维护（新增/修改回写、删除失效、refreshCache 整组清），无需靠 TTL 兜底。
	CacheTTLSysDict         = 0
	CacheTTLSysDictType     = 0
	CacheTTLSysClient       = 30 * 24 * time.Hour
	CacheTTLSysUserName     = 30 * 24 * time.Hour
	CacheTTLSysNickname     = 30 * 24 * time.Hour
	CacheTTLSysDept         = 30 * 24 * time.Hour
	CacheTTLSysRoleCustom   = 30 * 24 * time.Hour
	CacheTTLSysDeptAndChild = 30 * 24 * time.Hour
)

// 登录日志状态串，落 sys_login_info.status 前先经 logininfo 归类。
//
// auth 产生（登录成功/失败/登出），system 落库，故定义在消费并落库的这一侧。
const (
	LoginSuccess = "Success"
	Logout       = "Logout"
	Register     = "Register"
	LoginFail    = "Error"
)

// UserStatus 用户状态，落 sys_user.status。
//
// 用结构体而非裸常量是为了带上中文描述；判定时比 Code。
type UserStatus struct {
	Code string
	Info string
}

// 用户状态枚举实例。
var (
	UserStatusOK      = UserStatus{Code: "0", Info: "正常"}
	UserStatusDisable = UserStatus{Code: "1", Info: "停用"}
	UserStatusDeleted = UserStatus{Code: "2", Info: "删除"}
)
