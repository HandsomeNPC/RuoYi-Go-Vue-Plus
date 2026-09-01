package bo

// SysDictTypeQueryBo 字典类型列表查询条件（query 参数）。
//
// 与 SysDictTypeBo 分开而非复用：查询条件全部可选，而 SysDictTypeBo 的
// binding:"required" 与字典类型正则是新增场景的契约——按它校验会让
// "按类型前缀模糊搜索"（如 sys_）被正则挡掉。
//
// 无创建时间区间：Java buildQueryWrapper 未提供，不构造无效的查询能力。
type SysDictTypeQueryBo struct {
	DictName string `form:"dictName"`
	DictType string `form:"dictType"`
}
