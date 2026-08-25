package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysUserVo 用户信息视图对象，对应 Java SysUserVo。
type SysUserVo struct {
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
	AvatarURL string `json:"avatarUrl"`
	Password  string `json:"-"`
	// Status 账号状态（0正常 1停用）。
	Status     string     `json:"status"`
	LoginIP    string     `json:"loginIp"`
	LoginDate  *time.Time `json:"loginDate"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
	UpdateTime *time.Time `json:"updateTime"`
	// DeptName 部门名，由翻译层按 DEPT_ID_TO_NAME 从 DeptId 回填。
	DeptName string `json:"deptName"`
	// Roles 角色对象，由 service 层回填。
	Roles []SysRoleVo `json:"roles"`
	// RoleIDs 角色组，由 service 层回填。
	RoleIDs []int64 `json:"roleIds"`
	// PostIDs 岗位组，由 service 层回填。
	PostIDs []int64 `json:"postIds"`
	// RoleID 数据权限当前角色ID，由 service 层回填。
	RoleID int64 `json:"roleId"`
}

// FromSysUser 把实体转成 VO。
func FromSysUser(u *systemmodel.SysUser) *SysUserVo {
	if u == nil {
		return nil
	}
	return &SysUserVo{
		UserID:      u.UserID,
		DeptID:      u.DeptID,
		UserName:    u.UserName,
		NickName:    u.NickName,
		UserType:    u.UserType,
		Email:       u.Email,
		PhoneNumber: u.PhoneNumber,
		Gender:      u.Gender,
		Avatar:      u.Avatar,
		Password:    u.Password,
		Status:      u.Status,
		LoginIP:     u.LoginIP,
		LoginDate:   u.LoginDate,
		Remark:      u.Remark,
		CreateTime:  u.CreateTime,
		UpdateTime:  u.UpdateTime,
	}
}
