package dto

// RoleDTO 角色简要信息对象，对应 Java org.dromara.system.api.domain.RoleDTO。
type RoleDTO struct {
	// RoleID 角色ID。
	RoleID int64 `json:"roleId"`
	// RoleName 角色名称。
	RoleName string `json:"roleName"`
	// RoleKey 角色权限。
	RoleKey string `json:"roleKey"`
	// DataScope 数据范围（1：全部数据权限 2：自定数据权限 3：本部门数据权限
	// 4：本部门及以下数据权限 5：仅本人数据权限 6：部门及以下或本人数据权限）。
	DataScope string `json:"dataScope"`
}
