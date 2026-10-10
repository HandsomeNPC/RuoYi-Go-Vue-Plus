package vo

import (
	"github.com/HandsomeNPC/ruoyi-common-go/treeutil"
)

// DeptTreeSelectVo 角色部门树及其选中节点。
//
// CheckedKeys 用裸 []int64：雪花 ID 的 JSON 形态由 pkg/jsonx 的全局 codec 按值兜住。
// 不加 omitempty——前端按 checkedKeys.length 遍历，缺键会变成 undefined 而非空数组。
type DeptTreeSelectVo struct {
	CheckedKeys []int64                 `json:"checkedKeys"`
	Depts       []*treeutil.Tree[int64] `json:"depts"`
}
