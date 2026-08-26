package vo

import (
	"time"
)

// SysUserExportVo 用户对象导出视图对象，对应 Java SysUserExportVo。
type SysUserExportVo struct {
	UserID      int64  `json:"userId"`
	UserName    string `json:"userName"`
	DeptID      int64  `json:"deptId"`
	NickName    string `json:"nickName"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	// Status 账号状态（0正常 1停用）。
	Status    string     `json:"status"`
	LoginIP   string     `json:"loginIp"`
	LoginDate *time.Time `json:"loginDate"`
	// LeaderName 部门负责人名，由导出 service 层按部门关系回填。
	LeaderName string `json:"leaderName"`
}
