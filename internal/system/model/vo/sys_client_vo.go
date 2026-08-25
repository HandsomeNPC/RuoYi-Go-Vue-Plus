package vo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

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

// FromSysClient 把实体转成 VO。
func FromSysClient(c *systemmodel.SysClient) *SysClientVo {
	if c == nil {
		return nil
	}
	return &SysClientVo{
		ID:            c.ID,
		ClientID:      c.ClientID,
		ClientKey:     c.ClientKey,
		ClientSecret:  c.ClientSecret,
		GrantType:     c.GrantType,
		DeviceType:    c.DeviceType,
		AccessPath:    c.AccessPath,
		IPWhitelist:   c.IPWhitelist,
		ActiveTimeout: c.ActiveTimeout,
		Timeout:       c.Timeout,
		Status:        c.Status,
	}
}
