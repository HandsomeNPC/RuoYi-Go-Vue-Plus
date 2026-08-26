package model

import (
	"time"

	"ruoyi-go-vue-plus/pkg/repository"
)

// SysUser 用户信息表。
type SysUser struct {
	UserID   int64  `gorm:"column:user_id;primaryKey" json:"userId"`
	DeptID   int64  `gorm:"column:dept_id" json:"deptId"`
	UserName string `gorm:"column:user_name" json:"userName"`
	NickName string `gorm:"column:nick_name" json:"nickName"`
	// UserType 用户类型，取值见 enum.UserType。
	UserType    string `gorm:"column:user_type" json:"userType"`
	Email       string `gorm:"column:email" json:"email"`
	PhoneNumber string `gorm:"column:phone_number" json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `gorm:"column:gender" json:"gender"`
	// Avatar 头像，存的是 OSS 文件 id。
	Avatar int64 `gorm:"column:avatar" json:"avatar"`

	// Password BCrypt 哈希，json:"-" 不出现在响应体里。
	Password string `gorm:"column:password" json:"-"`

	// Status 账号状态（0正常 1停用），取值见 enum.UserStatus。
	Status string `gorm:"column:status" json:"status"`

	LoginIP   string     `gorm:"column:login_ip" json:"loginIp"`
	LoginDate *time.Time `gorm:"column:login_date" json:"loginDate"`

	Remark string `gorm:"column:remark" json:"remark"`

	// LogicDelete 提供 del_flag 逻辑删除，查询/更新自动过滤已删除记录。
	repository.LogicDelete
	BaseEntity
}

// TableName 显式指定表名。
func (SysUser) TableName() string {
	return "sys_user"
}
