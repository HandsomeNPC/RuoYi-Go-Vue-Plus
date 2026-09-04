package bo

// SysUserPasswordBo 个人中心修改密码入参。
type SysUserPasswordBo struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}
