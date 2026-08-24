// Package model auth 模块数据模型：登录入参(dto) / 登录结果(vo) / 在线用户记录。
package model

// OnlineUser 在线用户记录，对应 Java 的 UserOnlineDTO。
//
// 序列化进 Redis（键 online_tokens:<token>），供阶段 3 的在线用户管理
// （查询、强退）读取。json 键对齐 Java 侧 UserOnlineDTO 的字段名。
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
