package enum

import "testing"

// 锚定全部枚举取值。这些值直接落库、也被前端依赖，改动即为破坏性变更。
//
// 另一层作用：枚举实例是 var 而非 const（Go 不支持 struct 常量），
// 若被某处误赋值，这个测试会失败。
func TestEnumValues(t *testing.T) {
	codes := map[string]string{
		UserStatusOK.Code:      "0",
		UserStatusDisable.Code: "1",
		UserStatusDeleted.Code: "2",

		UserTypeSys.Code: "sys_user",
		UserTypeApp.Code: "app_user",

		DevicePC.Code:     "pc",
		DeviceApp.Code:    "app",
		DeviceXcx.Code:    "xcx",
		DeviceSocial.Code: "social",

		LoginTypePassword.Code: "password",
		LoginTypeSms.Code:      "sms",
		LoginTypeEmail.Code:    "email",
		LoginTypeSocial.Code:   "social",
		LoginTypeXcx.Code:      "xcx",
	}
	for got, want := range codes {
		if got != want {
			t.Errorf("Code = %q, want %q", got, want)
		}
	}

	if got, want := UserStatusOK.Info, "正常"; got != want {
		t.Errorf("UserStatusOK.Info = %q, want %q", got, want)
	}
	if got, want := UserStatusDisable.Info, "停用"; got != want {
		t.Errorf("UserStatusDisable.Info = %q, want %q", got, want)
	}
}

// Parse 系列按 Code 精确匹配，未命中返回 ok=false 且不得返回可用的零值。
func TestParseExact(t *testing.T) {
	if s, ok := ParseUserStatus("1"); !ok || s != UserStatusDisable {
		t.Errorf("ParseUserStatus(\"1\") = %v, %v", s, ok)
	}
	if _, ok := ParseUserStatus("9"); ok {
		t.Error("ParseUserStatus(\"9\") 应返回 false")
	}
	// 精确匹配不能退化为子串匹配 —— 这是与 ParseUserTypeFromLoginID 的关键区别。
	if _, ok := ParseUserType("sys_user:1"); ok {
		t.Error("ParseUserType 应精确匹配，不该接受 loginId")
	}
	if d, ok := ParseDeviceType("app"); !ok || d != DeviceApp {
		t.Errorf("ParseDeviceType(\"app\") = %v, %v", d, ok)
	}
	if lt, ok := ParseLoginType("password"); !ok || lt != LoginTypePassword {
		t.Errorf("ParseLoginType(\"password\") = %v, %v", lt, ok)
	}
	if _, ok := ParseLoginType(""); ok {
		t.Error("ParseLoginType(\"\") 应返回 false")
	}
}

// 从 loginId 提取用户类型走子串匹配（对齐原项目 UserType.getUserType）。
// 空串必须拦住：strings.Contains(s, "") 恒为 true，不拦会误判成 UserTypeSys。
func TestParseUserTypeFromLoginID(t *testing.T) {
	cases := []struct {
		loginID string
		want    UserType
		ok      bool
	}{
		{"sys_user:1", UserTypeSys, true},
		{"app_user:42", UserTypeApp, true},
		{"sys_user", UserTypeSys, true},
		{"", UserType{}, false},
		{"unknown:1", UserType{}, false},
	}
	for _, c := range cases {
		got, ok := ParseUserTypeFromLoginID(c.loginID)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseUserTypeFromLoginID(%q) = %v, %v; want %v, %v",
				c.loginID, got, ok, c.want, c.ok)
		}
	}
}

// 提示文案按登录方式渲染；social / xcx 不做重试计数，返回空串。
func TestLoginTypeMessages(t *testing.T) {
	if got, want := LoginTypePassword.RetryCountMsg(3), "密码输入错误3次"; got != want {
		t.Errorf("RetryCountMsg = %q, want %q", got, want)
	}
	if got, want := LoginTypePassword.RetryExceedMsg(5, 10), "密码输入错误5次，账户锁定10分钟"; got != want {
		t.Errorf("RetryExceedMsg = %q, want %q", got, want)
	}
	if got, want := LoginTypeSms.RetryCountMsg(2), "短信验证码输入错误2次"; got != want {
		t.Errorf("RetryCountMsg = %q, want %q", got, want)
	}
	if got, want := LoginTypeEmail.RetryExceedMsg(5, 10), "邮箱验证码输入错误5次，账户锁定10分钟"; got != want {
		t.Errorf("RetryExceedMsg = %q, want %q", got, want)
	}
	// 原项目 SocialAuthStrategy / XcxAuthStrategy 都不调 checkLogin，
	// 无重试文案，渲染须返回空串而不是 "%!d(MISSING)" 之类的脏字符串。
	for _, lt := range []LoginType{LoginTypeSocial, LoginTypeXcx} {
		if got := lt.RetryCountMsg(1); got != "" {
			t.Errorf("%s.RetryCountMsg = %q, want 空串", lt.Code, got)
		}
		if got := lt.RetryExceedMsg(5, 10); got != "" {
			t.Errorf("%s.RetryExceedMsg = %q, want 空串", lt.Code, got)
		}
	}
}

// LoginType.Code 的取值域须与原项目 grantType 一致 —— 5 种授权类型，
// 对应 ruoyi-admin 的 5 个 @Service("xxx"+IAuthStrategy.BASE_NAME) 策略 Bean。
//
// Java 的 LoginType 只有 4 个值（无 social），因为它仅用于取重试文案而
// social 不需要；Go 侧 Code 是查表键，必须覆盖全部 grantType。
func TestLoginTypeCoversAllGrantTypes(t *testing.T) {
	grantTypes := []string{"password", "sms", "email", "social", "xcx"}
	if len(loginTypes) != len(grantTypes) {
		t.Fatalf("登录方式数量 = %d, want %d", len(loginTypes), len(grantTypes))
	}
	for _, gt := range grantTypes {
		if _, ok := ParseLoginType(gt); !ok {
			t.Errorf("grantType %q 查不到对应 LoginType", gt)
		}
	}
}

// DeviceType 是参考取值而非白名单：sys_client.device_type 种子数据里就有
// "android" 这个不在枚举内的合法值。ok=false 不得被当作「设备类型非法」。
//
// 这个测试的作用是留档说明，防止后来者把 ParseDeviceType 用成登录校验 ——
// 那样 app 客户端（device_type='android'）会直接登录失败。
func TestParseDeviceTypeIsNotAWhitelist(t *testing.T) {
	if _, ok := ParseDeviceType("android"); ok {
		t.Error("枚举里不该有 android —— 若已补充，请同时修正类型注释与本测试")
	}
	if d, ok := ParseDeviceType("pc"); !ok || d != DevicePC {
		t.Errorf("ParseDeviceType(\"pc\") = %v, %v", d, ok)
	}
}

// Xxxs() 返回副本，调用方改动不得污染包内枚举表。
func TestListsReturnCopies(t *testing.T) {
	list := UserStatuses()
	if len(list) != 3 {
		t.Fatalf("UserStatuses() 长度 = %d, want 3", len(list))
	}
	list[0] = UserStatus{Code: "x"}
	if again := UserStatuses(); again[0] != UserStatusOK {
		t.Error("UserStatuses() 返回的不是副本，内部枚举表被污染")
	}

	types := UserTypes()
	types[0] = UserType{Code: "x"}
	if again := UserTypes(); again[0] != UserTypeSys {
		t.Error("UserTypes() 返回的不是副本")
	}

	devices := DeviceTypes()
	devices[0] = DeviceType{Code: "x"}
	if again := DeviceTypes(); again[0] != DevicePC {
		t.Error("DeviceTypes() 返回的不是副本")
	}

	logins := LoginTypes()
	if len(logins) != 5 {
		t.Fatalf("LoginTypes() 长度 = %d, want 5", len(logins))
	}
	logins[0] = LoginType{Code: "x"}
	if again := LoginTypes(); again[0] != LoginTypePassword {
		t.Error("LoginTypes() 返回的不是副本")
	}
}
