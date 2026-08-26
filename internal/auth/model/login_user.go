// Package model 鉴权域模型：登录态上下文、登录入参体等。
package model

import (
	"errors"
	"strconv"

	systemdto "ruoyi-go-vue-plus/internal/system/model/dto"
)

// LoginUser 登录用户上下文对象，保存当前会话的身份、权限和终端信息。
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
	// LoginTime 登录时间。
	LoginTime int64 `json:"loginTime"`
	// ExpireTime 过期时间。
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

// 对应 Java getLoginId() 抛出的 IllegalArgumentException 两种入参校验失败。
var (
	// ErrUserTypeEmpty 用户类型为空。
	ErrUserTypeEmpty = errors.New("用户类型不能为空")
	// ErrUserIDEmpty 用户ID为空。
	ErrUserIDEmpty = errors.New("用户ID不能为空")
)

// LoginID 获取会话使用的登录标识，形如 "sys_user:1761100000000000001"。
func (u *LoginUser) LoginID() (string, error) {
	if u == nil {
		return "", ErrUserTypeEmpty
	}
	if u.UserType == "" {
		return "", ErrUserTypeEmpty
	}
	if u.UserID == 0 {
		return "", ErrUserIDEmpty
	}
	return u.UserType + ":" + strconv.FormatInt(u.UserID, 10), nil
}
