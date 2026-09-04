package vo

import (
	"ruoyi-go-vue-plus/pkg/tree"
)

// MenuTreeSelectVo 角色菜单树及其选中节点。
//
// CheckedKeys 用裸 []int64：雪花 ID 的 JSON 形态由 pkg/jsonx 的全局 codec 按值兜住。
// 两个字段都不加 omitempty——前端按 checkedKeys.length 与 menus 遍历，
// 缺键会变成 undefined 而非空数组。
type MenuTreeSelectVo struct {
	CheckedKeys []int64             `json:"checkedKeys"`
	Menus       []*tree.Tree[int64] `json:"menus"`
}
