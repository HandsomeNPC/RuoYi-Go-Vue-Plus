// Package model system 模块数据模型：entity(表实体) / dto(入参) / vo(出参)。
package model

import "time"

// SysUser 用户信息表，对应原项目 org.dromara.system.domain.SysUser 与表 sys_user。
//
// 表结构见原项目 script/sql/ry_vue.sql:81-107，字段逐列对齐。
// GORM 的命名策略已配 SingularTable（见 pkg/database），故表名是 sys_user。
type SysUser struct {
	UserID   int64  `gorm:"column:user_id;primaryKey" json:"userId"`
	DeptID   int64  `gorm:"column:dept_id" json:"deptId"`
	UserName string `gorm:"column:user_name" json:"userName"`
	NickName string `gorm:"column:nick_name" json:"nickName"`
	// UserType 用户类型，取值见 enum.UserType。列默认值 'sys_user'。
	UserType    string `gorm:"column:user_type" json:"userType"`
	Email       string `gorm:"column:email" json:"email"`
	PhoneNumber string `gorm:"column:phone_number" json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `gorm:"column:gender" json:"gender"`
	// Avatar 头像，存的是 OSS 文件 id（不是 URL）—— 对齐 Java 侧的 Long 类型。
	Avatar int64 `gorm:"column:avatar" json:"avatar"`

	// Password BCrypt 哈希（$2a$10$ 开头，60 字符）。
	//
	// **json:"-" 是必须的**，对齐 Java SysUserVo 上
	// @JsonIgnore + @JsonProperty 那个「只读入不写出」的组合：
	// 登录时要把哈希读出来比对，但它绝不能出现在任何响应体里。
	// 少这个标签，任何返回 SysUser 的接口都会把密码哈希泄出去。
	Password string `gorm:"column:password" json:"-"`

	// Status 账号状态（0正常 1停用），取值见 enum.UserStatus。
	Status string `gorm:"column:status" json:"status"`
	// DelFlag 删除标志（0存在 1删除）。
	//
	// Java 侧靠 MyBatis-Plus 的 @TableLogic 自动给每条查询加
	// `del_flag = '0'`，GORM 没有等价机制 —— 由 repository 层的
	// NotDeleted() scope 统一施加，见 repository/scope.go。
	DelFlag string `gorm:"column:del_flag" json:"-"`

	LoginIP   string     `gorm:"column:login_ip" json:"loginIp"`
	LoginDate *time.Time `gorm:"column:login_date" json:"loginDate"`

	// 审计字段，对应 Java 的 BaseEntity。
	// Java 侧由 MyBatis-Plus 的字段填充器自动写入，Go 侧由各 repository
	// 在写操作里显式赋值（少一层隐式魔法，代价是要记得填）。
	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`

	Remark string `gorm:"column:remark" json:"remark"`
}

// TableName 显式指定表名。
//
// 命名策略已经能推导出 sys_user，这里仍写明是为了让「实体对应哪张表」
// 不依赖全局配置 —— 谁改了 SingularTable 也不会静默把表名变成 sys_users。
func (SysUser) TableName() string {
	return "sys_user"
}
