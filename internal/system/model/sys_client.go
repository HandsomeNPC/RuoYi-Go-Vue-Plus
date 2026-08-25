package model

// SysClient 系统授权表。纯 POJO，规则串的切分/归一化逻辑在 service 层。
type SysClient struct {
	ID        int64  `gorm:"column:id;primaryKey" json:"id"`
	ClientID  string `gorm:"column:client_id" json:"clientId"`
	ClientKey string `gorm:"column:client_key" json:"clientKey"`
	// ClientSecret 客户端密钥，不出现在响应体里。
	ClientSecret string `gorm:"column:client_secret" json:"-"`

	// GrantType 授权类型，逗号分隔的多值串。
	GrantType string `gorm:"column:grant_type" json:"grantType"`
	// DeviceType 设备类型，自由字符串。
	DeviceType string `gorm:"column:device_type" json:"deviceType"`

	// AccessPath 允许访问的路径规则，按 [,;\r\n]+ 分隔。空表示不限制。
	AccessPath string `gorm:"column:access_path" json:"accessPath"`
	// IPWhitelist 允许的来源 IP 规则。空表示不限制。
	IPWhitelist string `gorm:"column:ip_whitelist" json:"ipWhitelist"`

	// ActiveTimeout token 活跃超时（秒），滑动空闲超时。
	ActiveTimeout int64 `gorm:"column:active_timeout" json:"activeTimeout"`
	// Timeout token 固定超时（秒），落到 JWT 的 exp。
	Timeout int64 `gorm:"column:timeout" json:"timeout"`

	// Status 状态（0正常 1停用）。
	Status  string `gorm:"column:status" json:"status"`
	DelFlag string `gorm:"column:del_flag" json:"-"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysClient) TableName() string {
	return "sys_client"
}
