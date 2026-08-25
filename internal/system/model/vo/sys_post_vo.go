package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysPostVo 岗位信息视图对象，对应 Java SysPostVo。
type SysPostVo struct {
	PostID       int64  `json:"postId"`
	DeptID       int64  `json:"deptId"`
	PostCode     string `json:"postCode"`
	PostName     string `json:"postName"`
	PostCategory string `json:"postCategory"`
	PostSort     int    `json:"postSort"`
	// Status 状态（0正常 1停用）。
	Status     string     `json:"status"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
	// DeptName 部门名，由翻译层按 DEPT_ID_TO_NAME 从 DeptId 回填。
	DeptName string `json:"deptName"`
}

// FromSysPost 把实体转成 VO。
func FromSysPost(p *systemmodel.SysPost) *SysPostVo {
	if p == nil {
		return nil
	}
	return &SysPostVo{
		PostID:       p.PostID,
		DeptID:       p.DeptID,
		PostCode:     p.PostCode,
		PostName:     p.PostName,
		PostCategory: p.PostCategory,
		PostSort:     p.PostSort,
		Status:       p.Status,
		Remark:       p.Remark,
		CreateTime:   p.CreateTime,
	}
}
