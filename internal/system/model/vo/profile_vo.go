package vo

// ProfileVo 个人中心响应对象。
//
// 单独组装而非直接返回 SysUserVo：个人中心要避免敏感/脱敏字段，并附带角色组与岗位组。
type ProfileVo struct {
	User      *ProfileUserVo `json:"user"`
	RoleGroup string         `json:"roleGroup"`
	PostGroup string         `json:"postGroup"`
}
