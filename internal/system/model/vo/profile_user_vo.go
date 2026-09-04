package vo

import "time"

// ProfileUserVo 用户信息视图对象。
type ProfileUserVo struct {
	UserID   int64  `json:"userId"`
	DeptID   int64  `json:"deptId"`
	UserName string `json:"userName"`
	NickName string `json:"nickName"`
	// UserType 用户类型（sys_user 系统用户）。
	UserType    string `json:"userType"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	Avatar int64  `json:"avatar"`
	// AvatarURL 头像地址，由翻译层按 OSS_ID_TO_URL 从 Avatar 回填。
	AvatarURL string     `json:"avatarUrl"`
	LoginIP   string     `json:"loginIp"`
	LoginDate *time.Time `json:"loginDate"`
	// DeptName 部门名，由翻译层按 DEPT_ID_TO_NAME 从 DeptId 回填。
	DeptName string `json:"deptName"`
}
