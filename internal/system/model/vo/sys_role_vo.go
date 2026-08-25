package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysRoleVo 角色信息视图对象，对应 Java SysRoleVo。
type SysRoleVo struct {
	RoleID   int64  `json:"roleId"`
	RoleName string `json:"roleName"`
	RoleKey  string `json:"roleKey"`
	RoleSort int    `json:"roleSort"`
	// DataScope 数据范围（1全部 2自定 3本部门 4本部门及以下 5仅本人 6部门及以下或本人）。
	DataScope         string `json:"dataScope"`
	MenuCheckStrictly bool   `json:"menuCheckStrictly"`
	DeptCheckStrictly bool   `json:"deptCheckStrictly"`
	// Status 角色状态（0正常 1停用）。
	Status     string     `json:"status"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
	// Flag 用户是否存在此角色标识，默认 false，由 service 层回填。
	Flag bool `json:"flag"`
}

// FromSysRole 把实体转成 VO。
func FromSysRole(r *systemmodel.SysRole) *SysRoleVo {
	if r == nil {
		return nil
	}
	return &SysRoleVo{
		RoleID:            r.RoleID,
		RoleName:          r.RoleName,
		RoleKey:           r.RoleKey,
		RoleSort:          r.RoleSort,
		DataScope:         r.DataScope,
		MenuCheckStrictly: r.MenuCheckStrictly,
		DeptCheckStrictly: r.DeptCheckStrictly,
		Status:            r.Status,
		Remark:            r.Remark,
		CreateTime:        r.CreateTime,
	}
}
