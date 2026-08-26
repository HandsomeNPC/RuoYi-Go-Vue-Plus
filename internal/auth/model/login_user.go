// Package model 鉴权域模型：登录态上下文、登录入参体等。
package model

import (
	"strconv"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/pkg/enum"
)

// LoginUser 登录用户上下文对象，保存当前会话的身份、权限和终端信息。
// 对应 Java org.dromara.system.api.model.LoginUser；是存入 Redis 的会话载荷。
type LoginUser struct {
	// UserID 用户ID。
	UserID int64 `json:"userId"`
	// DeptID 部门ID。
	DeptID int64 `json:"deptId"`
	// DeptCategory 部门类别编码。
	DeptCategory string `json:"deptCategory"`
	// DeptName 部门名。
	DeptName string `json:"deptName"`
	// Token 用户唯一标识。
	Token string `json:"token"`
	// UserType 用户类型。
	UserType string `json:"userType"`
	// LoginTime 登录时间（毫秒）。
	LoginTime int64 `json:"loginTime"`
	// ExpireTime 过期时间（毫秒）。
	ExpireTime int64 `json:"expireTime"`
	// IPAddr 登录IP地址。
	IPAddr string `json:"ipaddr"`
	// LoginLocation 登录地点。
	LoginLocation string `json:"loginLocation"`
	// Browser 浏览器类型。
	Browser string `json:"browser"`
	// OS 操作系统。
	OS string `json:"os"`
	// MenuPermission 菜单权限。
	MenuPermission []string `json:"menuPermission"`
	// RolePermission 角色权限。
	RolePermission []string `json:"rolePermission"`
	// Username 用户名。
	Username string `json:"username"`
	// Nickname 用户昵称。
	Nickname string `json:"nickname"`
	// Roles 角色对象。
	Roles []*systemdto.RoleDTO `json:"roles"`
	// DataScopeRoleMap 数据权限角色映射，key 为权限码，value 为可参与数据权限计算的角色ID列表。
	DataScopeRoleMap map[string][]int64 `json:"dataScopeRoleMap"`
	// Posts 岗位对象。
	Posts []*systemdto.PostDTO `json:"posts"`
	// RoleID 数据权限当前角色ID。
	RoleID int64 `json:"roleId"`
	// ClientKey 客户端。
	ClientKey string `json:"clientKey"`
	// DeviceType 设备类型。
	DeviceType string `json:"deviceType"`
}

// LoginID 返回会话主体标识，形如 "sys_user:1761100000000000001"，
// 对应 Java LoginUser#getLoginId()；空则 ok=false（Go 版以错误值替代 Java 抛异常）。
func (u *LoginUser) LoginID() (string, bool) {
	if u == nil || u.UserType == "" || u.UserID == 0 {
		return "", false
	}
	return u.UserType + ":" + strconv.FormatInt(u.UserID, 10), true
}

// ParseLoginID 从会话主体标识中拆出用户类型与用户 ID。
func ParseLoginID(loginID string) (userType string, userID int64, ok bool) {
	for i := 0; i < len(loginID); i++ {
		if loginID[i] != ':' {
			continue
		}
		userType = loginID[:i]
		if userType == "" {
			return "", 0, false
		}
		id, err := strconv.ParseInt(loginID[i+1:], 10, 64)
		if err != nil || id == 0 {
			return "", 0, false
		}
		return userType, id, true
	}
	return "", 0, false
}

// IsSuperAdmin 判断是否超级管理员。
func (u *LoginUser) IsSuperAdmin() bool {
	return u != nil && u.UserID == superAdminUserID
}

// superAdminUserID 超级管理员用户 ID，与 constant.SuperAdminUserID 保持一致
// （由 login_user_test.go 的 TestSuperAdminIDMatchesConstant 在测试期防漂移）。
const superAdminUserID int64 = 1761100000000000001

// UserTypeSys 后台系统用户的类型标识。
var UserTypeSys = enum.UserTypeSys.Code
