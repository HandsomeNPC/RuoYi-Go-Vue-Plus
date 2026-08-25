package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysUserProfileBo 个人信息业务对象（入参），对应 Java SysUserProfileBo。
type SysUserProfileBo struct {
	NickName    string `json:"nickName" binding:"omitempty,max=30"`
	Email       string `json:"email" binding:"omitempty,email,max=50"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	Avatar int64  `json:"avatar"`
}

// ToSysUser 把 BO 转成实体。
func (b *SysUserProfileBo) ToSysUser() *systemmodel.SysUser {
	if b == nil {
		return nil
	}
	return &systemmodel.SysUser{
		NickName:    b.NickName,
		Email:       b.Email,
		PhoneNumber: b.PhoneNumber,
		Gender:      b.Gender,
		Avatar:      b.Avatar,
	}
}
