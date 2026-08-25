package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysUserBo 用户信息业务对象（入参），对应 Java SysUserBo。
type SysUserBo struct {
	UserID      int64  `json:"userId"`
	DeptID      int64  `json:"deptId"`
	UserName    string `json:"userName" binding:"required,min=2,max=30"`
	NickName    string `json:"nickName" binding:"required,max=30"`
	UserType    string `json:"userType"`
	Email       string `json:"email" binding:"omitempty,email,max=50"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	Avatar int64  `json:"avatar"`
	// Password 明文密码，由 service 转 BCrypt 后入库。
	Password string `json:"password"`
	// Status 账号状态（0正常 1停用）。
	Status string `json:"status"`
	Remark string `json:"remark"`
	// RoleIDs 角色组，不落 sys_user，由 UserService 写关联表。
	RoleIDs []int64 `json:"roleIds" binding:"required,min=1"`
	// PostIDs 岗位组，不落 sys_user，由 UserService 写关联表。
	PostIDs []int64 `json:"postIds"`
	// RoleID 数据权限当前角色ID，不落 sys_user，切换数据权限范围时用。
	RoleID int64 `json:"roleId"`
	// UserIDs 用户ID集合，不落 sys_user，工作流按用户范围审批时用。
	UserIDs string `json:"userIds"`
	// ExcludeUserIds 排除用户ID集合，不落 sys_user，工作流用。
	ExcludeUserIds string `json:"excludeUserIds"`
	CreateBy       int64  `json:"createBy"`
	UpdateBy       int64  `json:"updateBy"`
}

// ToSysUser 把 BO 转成实体。
func (b *SysUserBo) ToSysUser() *systemmodel.SysUser {
	if b == nil {
		return nil
	}
	return &systemmodel.SysUser{
		UserID:      b.UserID,
		DeptID:      b.DeptID,
		UserName:    b.UserName,
		NickName:    b.NickName,
		UserType:    b.UserType,
		Email:       b.Email,
		PhoneNumber: b.PhoneNumber,
		Gender:      b.Gender,
		Avatar:      b.Avatar,
		Password:    b.Password,
		Status:      b.Status,
		Remark:      b.Remark,
		BaseEntity: systemmodel.BaseEntity{
			CreateBy: b.CreateBy,
			UpdateBy: b.UpdateBy,
		},
	}
}
