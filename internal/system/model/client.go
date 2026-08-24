package model

import (
	"strings"
	"time"
)

// SysClient 系统授权表。
type SysClient struct {
	ID        int64  `gorm:"column:id;primaryKey" json:"id"`
	ClientID  string `gorm:"column:client_id" json:"clientId"`
	ClientKey string `gorm:"column:client_key" json:"clientKey"`
	// ClientSecret 客户端密钥，不出现在响应体里。
	ClientSecret string `gorm:"column:client_secret" json:"-"`

	// GrantType 授权类型，逗号分隔的多值串。用 GrantTypeList() 取切分后的列表。
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

	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`
}

// TableName 显式指定表名。
func (SysClient) TableName() string {
	return "sys_client"
}

// GrantTypeList 返回切分后的授权类型列表。
func (c SysClient) GrantTypeList() []string {
	return splitRules(c.GrantType)
}

// SupportsGrantType 判断该客户端是否支持指定授权类型（精确比对）。
func (c SysClient) SupportsGrantType(grantType string) bool {
	if grantType == "" {
		return false
	}
	for _, t := range c.GrantTypeList() {
		if t == grantType {
			return true
		}
	}
	return false
}

// AccessPathList 返回切分并归一化后的访问路径规则。
func (c SysClient) AccessPathList() []string {
	rules := splitRules(c.AccessPath)
	for i, r := range rules {
		rules[i] = normalizeAccessPath(r)
	}
	return rules
}

// IPWhitelistList 返回切分后的 IP 白名单规则（不做归一化）。
func (c SysClient) IPWhitelistList() []string {
	return splitRules(c.IPWhitelist)
}

// normalizeAccessPath 归一化单条访问路径规则。
func normalizeAccessPath(path string) string {
	if path == "*" || path == "/**" {
		return "/**"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// splitRules 按 , ; CR LF 切分规则串，trim 并丢弃空段。
func splitRules(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\r' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
