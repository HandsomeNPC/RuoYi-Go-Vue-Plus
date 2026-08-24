package model

import "time"

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
	// DelFlag 删除标志（0存在 1删除）。
	DelFlag string `gorm:"column:del_flag" json:"-"`

	LoginIP   string     `gorm:"column:login_ip" json:"loginIp"`
	LoginDate *time.Time `gorm:"column:login_date" json:"loginDate"`

	// 审计字段。
	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`

	Remark string `gorm:"column:remark" json:"remark"`
}

// TableName 显式指定表名。
func (SysUser) TableName() string {
	return "sys_user"
}
