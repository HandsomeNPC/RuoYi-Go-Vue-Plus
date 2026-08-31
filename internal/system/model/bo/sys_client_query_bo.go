package bo

// SysClientQueryBo 客户端列表查询条件（query 参数）。
//
// 与 SysClientBo 分开而非复用：查询条件全部可选，而 SysClientBo 的 binding:"required"
// 是新增场景的契约。Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则，
// 故按用途拆型。字段只取真正参与筛选的四个，其余列不构造无效的查询能力。
type SysClientQueryBo struct {
	ClientID     string `form:"clientId"`
	ClientKey    string `form:"clientKey"`
	ClientSecret string `form:"clientSecret"`
	// Status 状态（0正常 1停用）。
	Status string `form:"status"`
}
