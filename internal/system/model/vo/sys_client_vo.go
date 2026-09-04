package vo

// SysClientVo 授权管理视图对象。
type SysClientVo struct {
	ID           int64  `json:"id" excel:"id"`
	ClientID     string `json:"clientId" excel:"客户端id"`
	ClientKey    string `json:"clientKey" excel:"客户端key"`
	ClientSecret string `json:"clientSecret" excel:"客户端秘钥"`
	// GrantTypeList 授权类型组，不落 sys_client，由 service 切分 GrantType 回填。
	GrantTypeList []string `json:"grantTypeList"`
	GrantType     string   `json:"grantType" excel:"授权类型"`
	DeviceType    string   `json:"deviceType"`
	AccessPath    string   `json:"accessPath" excel:"允许访问路径"`
	// AccessPathList 访问路径组，不落 sys_client，由 service 切分 AccessPath 回填。
	AccessPathList []string `json:"accessPathList"`
	IPWhitelist    string   `json:"ipWhitelist" excel:"IP白名单"`
	// IPWhitelistList IP 白名单组，不落 sys_client，由 service 切分 IPWhitelist 回填。
	IPWhitelistList []string `json:"ipWhitelistList"`
	ActiveTimeout   int64    `json:"activeTimeout" excel:"token活跃超时时间"`
	Timeout         int64    `json:"timeout" excel:"token固定超时时间"`
	// Status 状态（0正常 1停用）。
	// 导出时按 excelDict 转成标签；客户端秘钥明文导出。
	Status string `json:"status" excel:"状态" excelDict:"0=正常,1=停用"`
}
