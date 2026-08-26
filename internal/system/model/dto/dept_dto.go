package dto

// DeptDTO 部门简要信息对象，对应 Java org.dromara.system.api.domain.DeptDTO。
type DeptDTO struct {
	// DeptID 部门ID。
	DeptID int64 `json:"deptId"`
	// ParentID 父部门ID。
	ParentID int64 `json:"parentId"`
	// DeptName 部门名称。
	DeptName string `json:"deptName"`
}
