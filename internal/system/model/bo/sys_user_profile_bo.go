package bo

// SysUserProfileBo 个人信息业务对象（入参）。
type SysUserProfileBo struct {
	NickName    string `json:"nickName" binding:"omitempty,max=30"`
	Email       string `json:"email" binding:"omitempty,email,max=50"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	Avatar int64  `json:"avatar"`
}
