// Package auth 登录态原语：LoginUser 模型、JWT 签发/校验、Redis 会话、密码哈希。
package auth

import (
	"strconv"

	"ruoyi-go-vue-plus/pkg/enum"
)

// LoginUser 登录用户信息，是存入 Redis 的会话载荷。
type LoginUser struct {
	UserID   int64  `json:"userId"`
	DeptID   int64  `json:"deptId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	UserType string `json:"userType"`

	DeptName     string `json:"deptName"`
	DeptCategory string `json:"deptCategory"`

	Token      string `json:"token"`
	LoginTime  int64  `json:"loginTime"`
	ExpireTime int64  `json:"expireTime"`

	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`

	MenuPermission []string `json:"menuPermission"`
	RolePermission []string `json:"rolePermission"`

	Roles            []RoleInfo         `json:"roles"`
	Posts            []PostInfo         `json:"posts"`
	DataScopeRoleMap map[string][]int64 `json:"dataScopeRoleMap"`
	RoleID           int64              `json:"roleId"`

	ClientKey  string `json:"clientKey"`
	DeviceType string `json:"deviceType"`
}

// RoleInfo 角色摘要。
type RoleInfo struct {
	RoleID    int64  `json:"roleId"`
	RoleName  string `json:"roleName"`
	RoleKey   string `json:"roleKey"`
	DataScope string `json:"dataScope"`
}

// PostInfo 岗位摘要。
type PostInfo struct {
	PostID   int64  `json:"postId"`
	PostName string `json:"postName"`
	PostCode string `json:"postCode"`
}

// LoginID 返回会话主体标识，形如 "sys_user:1761100000000000001"。
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

// superAdminUserID 超级管理员用户 ID。
const superAdminUserID int64 = 1761100000000000001

// UserTypeSys 后台系统用户的类型标识。
var UserTypeSys = enum.UserTypeSys.Code
