package dto

// UserOnlineDTO 当前在线会话信息对象。
type UserOnlineDTO struct {
	// TokenID 会话编号。
	TokenID string `json:"tokenId"`
	// DeptName 部门名称。
	DeptName string `json:"deptName"`
	// UserName 用户账号。
	UserName string `json:"userName"`
	// ClientKey 客户端。
	ClientKey string `json:"clientKey"`
	// DeviceType 设备类型。
	DeviceType string `json:"deviceType"`
	// IPAddr 登录IP地址。
	IPAddr string `json:"ipaddr"`
	// LoginLocation 登录地址。
	LoginLocation string `json:"loginLocation"`
	// Browser 浏览器类型。
	Browser string `json:"browser"`
	// OS 操作系统。
	OS string `json:"os"`
	// LoginTime 登录时间。
	LoginTime int64 `json:"loginTime"`
}
