package enum

import (
	"context"
	"testing"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// TestEnumValues 锚定全部枚举取值。
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

// TestParseExact 验证 Parse 系列按 Code 精确匹配。
func TestParseExact(t *testing.T) {
	if s, ok := ParseUserStatus("1"); !ok || s != UserStatusDisable {
		t.Errorf("ParseUserStatus(\"1\") = %v, %v", s, ok)
	}
	if _, ok := ParseUserStatus("9"); ok {
		t.Error("ParseUserStatus(\"9\") 应返回 false")
	}
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

// TestParseUserTypeFromLoginID 验证从 loginId 提取用户类型。
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

// TestLoginTypeMessages 验证提示文案按登录方式渲染。
func TestLoginTypeMessages(t *testing.T) {
	zh := i18n.NewContext(context.Background(), i18n.LocaleZhCN)
	en := i18n.NewContext(context.Background(), i18n.LocaleEnUS)

	countCases := []struct {
		name string
		lt   LoginType
		ctx  context.Context
		n    int
		want string
	}{
		{"密码-中文", LoginTypePassword, zh, 3, "密码输入错误3次"},
		{"密码-英文", LoginTypePassword, en, 3, "Password input error 3 times"},
		{"短信-中文", LoginTypeSms, zh, 2, "短信验证码输入错误2次"},
		{"短信-英文", LoginTypeSms, en, 2, "Sms code input error 2 times"},
		{"邮箱-中文", LoginTypeEmail, zh, 1, "邮箱验证码输入错误1次"},
	}
	for _, c := range countCases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.lt.RetryCountMsg(c.ctx, c.n); got != c.want {
				t.Errorf("RetryCountMsg = %q, want %q", got, c.want)
			}
		})
	}

	exceedCases := []struct {
		name              string
		lt                LoginType
		ctx               context.Context
		maxRetry, lockMin int
		want              string
	}{
		{"密码-中文", LoginTypePassword, zh, 5, 10, "密码输入错误5次，账户锁定10分钟"},
		{"密码-英文", LoginTypePassword, en, 5, 10, "Password input error 5 times, account locked for 10 minutes"},
		{"邮箱-中文", LoginTypeEmail, zh, 5, 10, "邮箱验证码输入错误5次，账户锁定10分钟"},
		{"邮箱-英文", LoginTypeEmail, en, 5, 10, "Email code input error 5 times, account locked for 10 minutes"},
	}
	for _, c := range exceedCases {
		t.Run("exceed/"+c.name, func(t *testing.T) {
			if got := c.lt.RetryExceedMsg(c.ctx, c.maxRetry, c.lockMin); got != c.want {
				t.Errorf("RetryExceedMsg = %q, want %q", got, c.want)
			}
		})
	}

	for _, lt := range []LoginType{LoginTypeSocial, LoginTypeXcx} {
		if got := lt.RetryCountMsg(zh, 1); got != "" {
			t.Errorf("%s.RetryCountMsg = %q, want 空串", lt.Code, got)
		}
		if got := lt.RetryExceedMsg(zh, 5, 10); got != "" {
			t.Errorf("%s.RetryExceedMsg = %q, want 空串", lt.Code, got)
		}
	}

	if got := LoginTypePassword.RetryCountMsg(context.Background(), 3); got != "密码输入错误3次" {
		t.Errorf("无语言 context 时 RetryCountMsg = %q, 应回落中文", got)
	}
}

// TestLoginTypeKeysExistInCatalog 验证词条键存在于词条表。
func TestLoginTypeKeysExistInCatalog(t *testing.T) {
	ctx := i18n.NewContext(context.Background(), i18n.LocaleZhCN)
	for _, lt := range LoginTypes() {
		for name, key := range map[string]string{
			"RetryCountKey":  lt.RetryCountKey,
			"RetryExceedKey": lt.RetryExceedKey,
		} {
			if key == "" {
				continue
			}
			if got := i18n.Msg(ctx, key); got == key {
				t.Errorf("%s.%s = %q 在词条表里不存在（Msg 原样返回了键）", lt.Code, name, key)
			}
		}
	}
}

// TestLoginTypeCoversAllGrantTypes 验证 Code 覆盖全部 grantType。
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

// TestParseDeviceTypeIsNotAWhitelist 验证枚举非白名单。
func TestParseDeviceTypeIsNotAWhitelist(t *testing.T) {
	if _, ok := ParseDeviceType("android"); ok {
		t.Error("枚举里不该有 android —— 若已补充，请同时修正类型注释与本测试")
	}
	if d, ok := ParseDeviceType("pc"); !ok || d != DevicePC {
		t.Errorf("ParseDeviceType(\"pc\") = %v, %v", d, ok)
	}
}

// TestListsReturnCopies 验证 Xxxs() 返回副本。
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
