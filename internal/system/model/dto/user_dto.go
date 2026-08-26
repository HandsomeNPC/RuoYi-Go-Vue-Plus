package dto

import "time"

// UserDTO 用户简要信息对象，对应 Java org.dromara.system.api.domain.UserDTO。
type UserDTO struct {
	// UserID 用户ID。
	UserID int64 `json:"userId"`
	// DeptID 部门ID。
	DeptID int64 `json:"deptId"`
	// UserName 用户账号。
	UserName string `json:"userName"`
	// NickName 用户昵称。
	NickName string `json:"nickName"`
	// UserType 用户类型（sys_user系统用户）。
	UserType string `json:"userType"`
	// Email 用户邮箱。
	Email string `json:"email"`
	// PhoneNumber 手机号码。
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	// Status 账号状态（0正常 1停用）。
	Status string `json:"status"`
	// CreateTime 创建时间。
	CreateTime *time.Time `json:"createTime"`
}
