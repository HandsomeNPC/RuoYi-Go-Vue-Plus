package model

// OnlineUser 在线用户记录。
type OnlineUser struct {
	TokenID       string `json:"tokenId"`
	UserName      string `json:"userName"`
	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`
	DeptName      string `json:"deptName"`
	ClientKey     string `json:"clientKey"`
	DeviceType    string `json:"deviceType"`
	LoginTime     int64  `json:"loginTime"`
}
