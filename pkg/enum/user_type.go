package enum

import "strings"

// UserType 用户类型，对应原项目 enums.UserType。
//
// 同一套用户表可承载多种用户体系。登录态标识 loginId 形如 "sys_user:1"，
// 由 用户类型 + ":" + 用户 ID 拼成，用于区分后台用户与 App 用户的权限体系。
type UserType struct {
	Code string // 用户类型标识，用于 token、权限识别
}

// 用户类型枚举实例。
var (
	UserTypeSys = UserType{Code: "sys_user"} // 后台系统用户
	UserTypeApp = UserType{Code: "app_user"} // 移动客户端用户
)

// userTypes 全部用户类型，顺序与原枚举声明一致。
//
// ParseUserTypeFromLoginID 依赖此顺序做子串匹配，若将来新增类型且标识之间
// 存在包含关系（如 "app_user" 与 "app_user_v2"），须把更具体的排在前面。
var userTypes = []UserType{UserTypeSys, UserTypeApp}

// UserTypes 返回全部用户类型的副本。
func UserTypes() []UserType {
	return append([]UserType(nil), userTypes...)
}

// ParseUserType 按 Code 精确查找用户类型，未匹配时 ok 为 false。
func ParseUserType(code string) (UserType, bool) {
	for _, t := range userTypes {
		if t.Code == code {
			return t, true
		}
	}
	return UserType{}, false
}

// ParseUserTypeFromLoginID 从登录标识中提取用户类型，对应原项目
// UserType.getUserType(String)。
//
// 原方法用 StringUtils.contains 做**子串**匹配（而非精确相等），因为传入的是
// loginId "sys_user:1" 这类拼接串。这里保留同样的语义，但与原方法有两点不同：
//
//   - 原方法匹配失败抛 RuntimeException，这里返回 ok=false 交调用方处置，
//     符合 Go 惯例，也避免在中间件里因为一个畸形 token 就 panic。
//   - 空串直接返回 false。strings.Contains(s, "") 恒为 true，若不拦住，
//     空 loginId 会被误判成 UserTypeSys —— 这是移植时的隐性陷阱。
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
