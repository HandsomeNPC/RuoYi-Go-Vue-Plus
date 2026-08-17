// Package systemconstants 系统常量信息
//
// @author Lion Li
package systemconstants

const (
	// Normal 正常状态
	Normal = "0"

	// Disable 异常状态
	Disable = "1"

	// Yes 是
	Yes = "Y"

	// No 否
	No = "N"

	// TypeDir 菜单类型（目录）
	TypeDir = "M"

	// TypeMenu 菜单类型（菜单）
	TypeMenu = "C"

	// TypeButton 菜单类型（按钮）
	TypeButton = "F"

	// Layout Layout组件标识
	Layout = "Layout"

	// ParentView ParentView组件标识
	ParentView = "ParentView"

	// InnerLink InnerLink组件标识
	InnerLink = "InnerLink"

	// SuperAdminUserID 超级管理员用户ID
	SuperAdminUserID int64 = 1761100000000000001

	// SuperAdminRoleID 超级管理员角色ID
	SuperAdminRoleID int64 = 1761300000000000001

	// SuperAdminRoleKey 超级管理员角色 roleKey
	SuperAdminRoleKey = "superadmin"

	// RootDeptAncestors 根部门祖级列表
	RootDeptAncestors = "0"

	// DefaultDeptID 默认部门 ID
	DefaultDeptID int64 = 1761000000000000100
)

// ExcludeProperties 排除敏感属性字段
var ExcludeProperties = []string{"password", "oldPassword", "newPassword", "confirmPassword"}
