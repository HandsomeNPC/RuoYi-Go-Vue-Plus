package dto

// LoginInfoEvent 登录事件。
//
// 本项目无事件总线，改由 auth 直接调 system 的 LoginInfoSvcApp.RecordLoginInfo
// （同进程函数调用，内部异步落库），此结构体退化为纯入参载体。
type LoginInfoEvent struct {
	// Username 用户账号。
	Username string `json:"username"`
	// Status 登录状态，取 constant.ConstantLoginSuccess / ConstantLogout /
	// ConstantRegister / ConstantLoginFail 之一（注意不是落表的 0/1，
	// 落表值由 RecordLoginInfo 映射）。
	Status string `json:"status"`
	// Message 提示消息。
	Message string `json:"message"`
	// IP 客户端IP。
	IP string `json:"ip"`
	// UserAgent 用户代理。
	UserAgent string `json:"userAgent"`
	// ClientID 客户端标识（sys_client.client_id），用于反查 ClientKey/DeviceType。
	ClientID string `json:"clientId"`
}
