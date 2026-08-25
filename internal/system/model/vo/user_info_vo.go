package vo

// UserInfoVo 登录用户信息视图对象，对应 Java UserInfoVo。
type UserInfoVo struct {
	User        SysUserVo `json:"user"`
	Permissions []string  `json:"permissions"`
	Roles       []string  `json:"roles"`
}
