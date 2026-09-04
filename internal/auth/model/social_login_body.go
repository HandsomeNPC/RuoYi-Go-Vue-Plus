package model

// SocialLoginBody 三方平台绑定请求对象。
//
// 不嵌 LoginBody：绑定接口只认当前登录态，用不到 clientId/grantType；
// 且 socialCallback 未对该接口校验这两个必填字段，嵌进来反而会拒掉本应能收的请求。
type SocialLoginBody struct {
	// Source 三方登录平台，如 gitee / github。
	Source string `json:"source" binding:"required"`
	// SocialCode 三方返回的授权码。
	SocialCode string `json:"socialCode" binding:"required"`
	// SocialState 三方回传的 state，须与授权时下发的一致。
	SocialState string `json:"socialState" binding:"required"`
}
