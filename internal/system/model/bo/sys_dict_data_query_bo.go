package bo

// SysDictDataQueryBo 字典数据列表查询条件（query 参数）。
//
// 与 SysDictDataBo 分开而非复用：查询条件全部可选，而 SysDictDataBo 的
// binding:"required" 是新增场景的契约。Go 的 binding tag 没有校验分组概念，
// 一个结构体只能有一套规则。
//
// 无创建时间区间：Java buildQueryWrapper 未提供，前端筛选表单也只有字典标签，
// 不构造无效的查询能力。
type SysDictDataQueryBo struct {
	// DictSort 取 0 视为不筛。Java 用 Integer 的 null 区分"未传"与"显式传 0"，
	// Go 的 int 两者都塌成 0；排序号从 1 起算，取不到可观测差异。
	DictSort  int    `form:"dictSort"`
	DictLabel string `form:"dictLabel"`
	DictType  string `form:"dictType"`
}
