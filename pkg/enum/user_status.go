package enum

// UserStatus 用户状态，对应原项目 enums.UserStatus。
type UserStatus struct {
	Code string // 状态编码，落库到 sys_user.status
	Info string // 状态说明，用于提示与展示
}

// 用户状态枚举实例。
//
// UserStatusDeleted 原项目保留了定义但实际不会出现在库里 —— 删除走
// del_flag 逻辑删除，status 不会被置为 "2"。移植过来仅为对齐取值。
var (
	UserStatusOK      = UserStatus{Code: "0", Info: "正常"}
	UserStatusDisable = UserStatus{Code: "1", Info: "停用"}
	UserStatusDeleted = UserStatus{Code: "2", Info: "删除"}
)

// userStatuses 全部用户状态，顺序与原枚举声明一致。
var userStatuses = []UserStatus{UserStatusOK, UserStatusDisable, UserStatusDeleted}

// UserStatuses 返回全部用户状态的副本。
//
// 返回副本而非内部切片，避免调用方改动影响其他调用方 ——
// 这是 var 枚举必须自己守住的边界。
func UserStatuses() []UserStatus {
	return append([]UserStatus(nil), userStatuses...)
}

// ParseUserStatus 按 Code 精确查找用户状态，未匹配时 ok 为 false。
func ParseUserStatus(code string) (UserStatus, bool) {
	for _, s := range userStatuses {
		if s.Code == code {
			return s, true
		}
	}
	return UserStatus{}, false
}
