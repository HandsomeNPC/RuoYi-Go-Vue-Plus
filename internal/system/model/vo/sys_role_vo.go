package vo

import (
	"time"
)

// SysRoleVo 角色信息视图对象，对应 Java SysRoleVo。
type SysRoleVo struct {
	RoleID   int64  `json:"roleId" excel:"角色序号"`
	RoleName string `json:"roleName" excel:"角色名称"`
	RoleKey  string `json:"roleKey" excel:"角色权限"`
	RoleSort int    `json:"roleSort" excel:"角色排序"`
	// DataScope 数据范围（1全部 2自定 3本部门 4本部门及以下 5仅本人 6部门及以下或本人）。
	// 导出时按 excelDict 转标签，对齐 Java @ExcelDictFormat(readConverterExp = ...)。
	DataScope         string `json:"dataScope" excel:"数据范围" excelDict:"1=全部数据权限,2=自定义数据权限,3=本部门数据权限,4=本部门及以下数据权限,5=仅本人数据权限,6=部门及以下或本人数据权限"`
	MenuCheckStrictly bool   `json:"menuCheckStrictly" excel:"菜单树选择项是否关联显示"`
	DeptCheckStrictly bool   `json:"deptCheckStrictly" excel:"部门树选择项是否关联显示"`
	// Status 角色状态（0正常 1停用）。
	// 导出时按 excelDict 转标签，对齐 Java @ExcelDictFormat(dictType = "sys_normal_disable")。
	Status     string     `json:"status" excel:"角色状态" excelDict:"0=正常,1=停用"`
	Remark     string     `json:"remark" excel:"备注"`
	CreateTime *time.Time `json:"createTime" excel:"创建时间"`
	// Flag 用户是否存在此角色标识，默认 false，由 service 层回填。不导出。
	Flag bool `json:"flag"`
}
