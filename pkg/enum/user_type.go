package enum

import "strings"

// UserType 用户类型。
type UserType struct {
	Code string
}

// 用户类型枚举实例。
var (
	UserTypeSys = UserType{Code: "sys_user"}
	UserTypeApp = UserType{Code: "app_user"}
)

var userTypes = []UserType{UserTypeSys, UserTypeApp}

// UserTypes 返回全部用户类型的副本。
func UserTypes() []UserType {
	return append([]UserType(nil), userTypes...)
}

// ParseUserType 按 Code 查找用户类型，未匹配时 ok 为 false。
func ParseUserType(code string) (UserType, bool) {
	for _, t := range userTypes {
		if t.Code == code {
			return t, true
		}
	}
	return UserType{}, false
}

// ParseUserTypeFromLoginID 从登录标识中提取用户类型，未匹配时 ok 为 false。
func ParseUserTypeFromLoginID(loginID string) (UserType, bool) {
	if loginID == "" {
		return UserType{}, false
	}
	for _, t := range userTypes {
		if strings.Contains(loginID, t.Code) {
			return t, true
		}
	}
	return UserType{}, false
}
