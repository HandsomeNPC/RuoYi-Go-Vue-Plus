package enum

// UserStatus 用户状态。
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

var userStatuses = []UserStatus{UserStatusOK, UserStatusDisable, UserStatusDeleted}

// UserStatuses 返回全部用户状态的副本。
func UserStatuses() []UserStatus {
	return append([]UserStatus(nil), userStatuses...)
}

// ParseUserStatus 按 Code 查找用户状态，未匹配时 ok 为 false。
func ParseUserStatus(code string) (UserStatus, bool) {
	for _, s := range userStatuses {
		if s.Code == code {
			return s, true
		}
	}
	return UserStatus{}, false
}
