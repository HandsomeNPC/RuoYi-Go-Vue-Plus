package bo

// SysClientBo 授权管理业务对象（入参），对应 Java SysClientBo。
type SysClientBo struct {
	ID           int64  `json:"id"`
	ClientID     string `json:"clientId"`
	ClientKey    string `json:"clientKey" binding:"required"`
	ClientSecret string `json:"clientSecret" binding:"required"`
	// GrantTypeList 授权类型组，不落 sys_client，由 ClientService join 成 GrantType 入库。
	GrantTypeList []string `json:"grantTypeList" binding:"required"`
	GrantType     string   `json:"grantType"`
	DeviceType    string   `json:"deviceType"`
	AccessPath    string   `json:"accessPath"`
	// AccessPathList 访问路径组，不落 sys_client，由 ClientService join 成 AccessPath 入库。
	AccessPathList []string `json:"accessPathList"`
	IPWhitelist    string   `json:"ipWhitelist"`
	// IPWhitelistList IP 白名单组，不落 sys_client，由 ClientService join 成 IPWhitelist 入库。
	IPWhitelistList []string `json:"ipWhitelistList"`
	ActiveTimeout   int64    `json:"activeTimeout"`
	Timeout         int64    `json:"timeout"`
	// Status 状态（0正常 1停用）。
	Status string `json:"status"`
}
