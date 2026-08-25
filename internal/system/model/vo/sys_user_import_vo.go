package vo

// SysUserImportVo 用户对象导入视图对象，对应 Java SysUserImportVo。
type SysUserImportVo struct {
	UserID      int64  `json:"userId"`
	DeptID      int64  `json:"deptId"`
	UserName    string `json:"userName"`
	NickName    string `json:"nickName"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender"`
	// Status 账号状态（0正常 1停用）。
	Status string `json:"status"`
}
