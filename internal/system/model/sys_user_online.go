package model

// SysUserOnline 当前在线会话，对应 Java org.dromara.system.domain.SysUserOnline。
//
// 非表实体：数据来源于 Redis 中活跃的 token 会话，故无 gorm 列标签与 TableName。
type SysUserOnline struct {
	TokenID       string `json:"tokenId"`
	DeptName      string `json:"deptName"`
	UserName      string `json:"userName"`
	ClientKey     string `json:"clientKey"`
	DeviceType    string `json:"deviceType"`
	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`
	LoginTime     int64  `json:"loginTime"`
}
