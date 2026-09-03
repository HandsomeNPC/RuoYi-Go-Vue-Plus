package model

// SocialLoginBody 三方平台绑定请求对象（对应 Java SocialLoginBody）。
//
// 不嵌 LoginBody：Java 的 SocialLoginBody 继承它是为了兼作三方「登录」入参
// （grantType=social 走 SocialAuthStrategy），而绑定接口只认当前登录态，
// 用不到 clientId/grantType。且 Java 的 socialCallback 没标 @Validated，
// 那两个 @NotBlank 在该接口上根本不生效，嵌进来反而会拒掉 Java 能收的请求。
type SocialLoginBody struct {
	// Source 三方登录平台，如 gitee / github。
	Source string `json:"source" binding:"required"`
	// SocialCode 三方返回的授权码。
	SocialCode string `json:"socialCode" binding:"required"`
	// SocialState 三方回传的 state，须与授权时下发的一致。
	SocialState string `json:"socialState" binding:"required"`
}
