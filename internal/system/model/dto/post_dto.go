package dto

// PostDTO 岗位简要信息对象，对应 Java org.dromara.system.api.domain.PostDTO。
type PostDTO struct {
	// PostID 岗位ID。
	PostID int64 `json:"postId"`
	// DeptID 部门id。
	DeptID int64 `json:"deptId"`
	// PostCode 岗位编码。
	PostCode string `json:"postCode"`
	// PostName 岗位名称。
	PostName string `json:"postName"`
	// PostCategory 岗位类别编码。
	PostCategory string `json:"postCategory"`
}
