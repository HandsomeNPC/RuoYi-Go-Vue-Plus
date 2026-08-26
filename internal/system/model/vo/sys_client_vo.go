package vo

// SysClientVo 授权管理视图对象，对应 Java SysClientVo。
type SysClientVo struct {
	ID           int64  `json:"id"`
	ClientID     string `json:"clientId"`
	ClientKey    string `json:"clientKey"`
	ClientSecret string `json:"clientSecret"`
	// GrantTypeList 授权类型组，不落 sys_client，由 service 切分 GrantType 回填。
	GrantTypeList []string `json:"grantTypeList"`
	GrantType     string   `json:"grantType"`
	DeviceType    string   `json:"deviceType"`
	AccessPath    string   `json:"accessPath"`
	// AccessPathList 访问路径组，不落 sys_client，由 service 切分 AccessPath 回填。
	AccessPathList []string `json:"accessPathList"`
	IPWhitelist    string   `json:"ipWhitelist"`
	// IPWhitelistList IP 白名单组，不落 sys_client，由 service 切分 IPWhitelist 回填。
	IPWhitelistList []string `json:"ipWhitelistList"`
	ActiveTimeout   int64    `json:"activeTimeout"`
	Timeout         int64    `json:"timeout"`
	// Status 状态（0正常 1停用）。
	Status string `json:"status"`
}
