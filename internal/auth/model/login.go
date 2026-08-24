// Package model auth 模块数据模型：登录入参(dto) / 登录结果(vo)。
package model

// LoginBody 登录入参，对应原项目 LoginBody + PasswordLoginBody 两个类。
//
// Java 侧分两层是因为 AuthController 先按 clientId/grantType 分派策略、
// 再由具体策略解析自己需要的字段（body 被解析两次，见 AuthController.java:77）。
// Go 侧合成一个结构体：一次绑定拿全，少一次 JSON 解析。
// 代价是短信/邮箱登录（阶段 4）的字段将来也要加进来 —— 届时若字段互斥
// 得难看，再按 grantType 拆成多个结构体。
//
// # 校验规则对齐 Java 的注解
//
// binding 标签逐条对应原项目的 @NotBlank / @Length：
//
//	clientId  @NotBlank                  auth.clientid.not.blank
//	grantType @NotBlank                  auth.grant.type.not.blank
//	username  @NotBlank @Length(2,30)    user.username.not.blank / length.valid
//	password  @NotBlank @Length(5,30)    user.password.not.blank / length.valid
//
// **密码复杂度校验有意不加**：原项目的 @Pattern(regexp = RegexConstants.PASSWORD)
// 在 PasswordLoginBody.java 里是**注释掉的**。加上它会让存量的简单密码
// （种子数据里就是 admin123 / 666666）全部登录失败 —— 那不是「更安全」，
// 是把所有人锁在门外。要收紧密码策略得在改密码入口做，不在登录入口。
type LoginBody struct {
	// ClientID 客户端标识 = MD5(clientKey + clientSecret)。
	ClientID string `json:"clientId" binding:"required"`
	// GrantType 授权类型，取值见 enum.LoginType 的 Code（本阶段只支持 password）。
	GrantType string `json:"grantType" binding:"required"`

	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=5,max=30"`

	// Code / UUID 验证码与其标识。
	//
	// **本阶段不校验**：原项目 captcha.enable 默认 false
	// （application.yml:20），且验证码生成接口属阶段 3。
	// 字段先留着，让前端的既有报文能原样绑定成功 ——
	// binding 无 required，不传也不影响。
	Code string `json:"code"`
	UUID string `json:"uuid"`
}

// LoginVo 登录结果，对应原项目 org.dromara.web.domain.vo.LoginVo。
//
// **json 键是下划线风格**，对齐 Java 侧那 5 个 @JsonProperty
// （access_token / refresh_token / expire_in / refresh_expire_in / client_id）——
// 这是与前端的协议，改成驼峰前端就取不到 token 了。
//
// 密码登录只填 AccessToken / ExpireIn / ClientID 三项，其余恒为空 ——
// 对齐 Java 的 PasswordAuthStrategy（那边同样只 set 这三个）。
// RefreshToken 那套在原项目里从未被任何策略填充过，保留字段是为了
// 前端若按完整结构解析不会缺键。
type LoginVo struct {
	AccessToken string `json:"access_token"`
	// ExpireIn token 剩余有效期（秒），取自 sys_client.timeout。
	// 对应 Java 的 StpUtil.getTokenTimeout()。
	ExpireIn int64  `json:"expire_in"`
	ClientID string `json:"client_id"`

	// 以下均为原项目保留但未启用的字段。
	RefreshToken    string `json:"refresh_token,omitempty"`
	RefreshExpireIn int64  `json:"refresh_expire_in,omitempty"`
	Scope           string `json:"scope,omitempty"`
	OpenID          string `json:"openid,omitempty"`
}
