package constant

import (
	"strings"
	"testing"
	"time"
)

// TestPatternsAnchored 验证正则整串匹配。
func TestPatternsAnchored(t *testing.T) {
	cases := []struct {
		name  string
		valid []string
		bad   []string
		match func(string) bool
	}{
		{
			name:  "DictionaryType",
			valid: []string{"sys_user_sex", "a", "a1_"},
			bad:   []string{"", "1abc", "Sys_user", "sys-user", "x sys_user y"},
			match: PatternDictionaryType.MatchString,
		},
		{
			name:  "PermissionString",
			valid: []string{"", "system:user:list", "system:user:*", "tool:build:list"},
			bad:   []string{"system:user", "*:user:list", "system:user:list:extra", " system:user:list"},
			match: PatternPermissionString.MatchString,
		},
		{
			name:  "Mobile",
			valid: []string{"13800138000", "013800138000", "8613800138000", "+8613800138000"},
			bad:   []string{"12800138000", "1380013800", "abc13800138000", "13800138000xyz"},
			match: PatternMobile.MatchString,
		},
		{
			name:  "IDCardLast6",
			valid: []string{"01123X", "31012x", "291234", "101234"},
			bad:   []string{"", "40123X", "00123X", "32123X", "0112X", "01123XY", "a01123X"},
			match: PatternIDCardLast6.MatchString,
		},
		{
			name:  "QQNumber",
			valid: []string{"100000", "1234567890", "12345678901"},
			bad:   []string{"", "012345", "10000", "123456789012", "12345a"},
			match: PatternQQNumber.MatchString,
		},
		{
			name:  "PostalCode",
			valid: []string{"100000", "310000"},
			bad:   []string{"", "010000", "10000", "1000000", "10000a"},
			match: PatternPostalCode.MatchString,
		},
		{
			name:  "Account",
			valid: []string{"admin", "a1234", "abcdefghijklmnop"},
			bad:   []string{"abcd", "1admin", "abcdefghijklmnopq", "adm in"},
			match: PatternAccount.MatchString,
		},
		{
			name:  "Status",
			valid: []string{StatusNormal, StatusDisable},
			bad:   []string{"", "2", "01", "0a"},
			match: PatternStatus.MatchString,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, s := range c.valid {
				if !c.match(s) {
					t.Errorf("%q 应通过校验但被拒绝", s)
				}
			}
			for _, s := range c.bad {
				if c.match(s) {
					t.Errorf("%q 应被拒绝但通过了校验", s)
				}
			}
		})
	}
}

// TestValidPassword 校验密码规则。
func TestValidPassword(t *testing.T) {
	valid := []string{"Abcdef1!", "aB3$aB3$", "P@ssw0rdP@ssw0rd"}
	bad := []string{
		"",
		"Abcd1!a",
		"abcdef1!",
		"ABCDEF1!",
		"Abcdefg!",
		"Abcdefg1",
		"Abcdef1!#",
		"Abcdef1!中",
		"Abcdef1! ",
	}
	for _, s := range valid {
		if !ValidPassword(s) {
			t.Errorf("ValidPassword(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if ValidPassword(s) {
			t.Errorf("ValidPassword(%q) = true, want false", s)
		}
	}
}

// TestGlobalKeyPrefixes 验证全局 key 前缀。
func TestGlobalKeyPrefixes(t *testing.T) {
	prefixes := map[string]string{
		CaptchaCodeKey:    "global:captcha_codes:",
		RepeatSubmitKey:   "global:repeat_submit:",
		RateLimitKey:      "global:rate_limit:",
		SocialAuthCodeKey: "global:social_auth_codes:",
	}
	for got, want := range prefixes {
		if got != want {
			t.Errorf("key 前缀 = %q, want %q", got, want)
		}
		if !strings.HasPrefix(got, GlobalRedisKey) {
			t.Errorf("%q 不在 %q 命名空间下", got, GlobalRedisKey)
		}
	}
}

// TestStandaloneKeyPrefixes 验证独立 key 前缀。
func TestStandaloneKeyPrefixes(t *testing.T) {
	if got, want := OnlineTokenKeyPrefix+"tk", "online_tokens:tk"; got != want {
		t.Errorf("在线用户 key = %q, want %q", got, want)
	}
	if got, want := PwdErrCntKeyPrefix+"admin", "pwd_err_cnt:admin"; got != want {
		t.Errorf("密码错误次数 key = %q, want %q", got, want)
	}
}

// TestCacheGroupNamesHaveNoParams 验证缓存组名不含 '#' 参数。
func TestCacheGroupNamesHaveNoParams(t *testing.T) {
	groups := map[string]string{
		CacheDemoCache:       "demo:cache",
		CacheSysConfig:       "sys_config",
		CacheSysDict:         "sys_dict",
		CacheSysDictType:     "sys_dict_type",
		CacheSysClient:       "sys_client",
		CacheSysUserName:     "sys_user_name",
		CacheSysNickname:     "sys_nickname",
		CacheSysDept:         "sys_dept",
		CacheSysOss:          "sys_oss",
		CacheSysOssConfig:    "sys_oss_config",
		CacheSysRoleCustom:   "sys_role_custom",
		CacheSysDeptAndChild: "sys_dept_and_child",
	}
	for got, want := range groups {
		if got != want {
			t.Errorf("组名 = %q, want %q", got, want)
		}
		if strings.Contains(got, "#") {
			t.Errorf("组名 %q 不应含 '#'(Redisson 策略参数)", got)
		}
	}
}

// TestCacheTTL 验证缓存组 TTL。
func TestCacheTTL(t *testing.T) {
	ttls := map[string]struct {
		got  time.Duration
		want time.Duration
	}{
		"demo:cache": {CacheTTLDemoCache, 60 * time.Second},
		// 字典两组不过期。
		"sys_dict":           {CacheTTLSysDict, 0},
		"sys_dict_type":      {CacheTTLSysDictType, 0},
		"sys_client":         {CacheTTLSysClient, 720 * time.Hour},
		"sys_user_name":      {CacheTTLSysUserName, 720 * time.Hour},
		"sys_nickname":       {CacheTTLSysNickname, 720 * time.Hour},
		"sys_dept":           {CacheTTLSysDept, 720 * time.Hour},
		"sys_oss":            {CacheTTLSysOss, 720 * time.Hour},
		"sys_role_custom":    {CacheTTLSysRoleCustom, 720 * time.Hour},
		"sys_dept_and_child": {CacheTTLSysDeptAndChild, 720 * time.Hour},
	}
	for name, c := range ttls {
		if c.got != c.want {
			t.Errorf("%s TTL = %v, want %v", name, c.got, c.want)
		}
	}
}

// TestSystemConstants 验证系统常量取值。
func TestSystemConstants(t *testing.T) {
	if SuperAdminUserID != 1761100000000000001 {
		t.Errorf("SuperAdminUserID = %d", SuperAdminUserID)
	}
	if SuperAdminRoleID != 1761300000000000001 {
		t.Errorf("SuperAdminRoleID = %d", SuperAdminRoleID)
	}
	if DefaultDeptID != 1761000000000000100 {
		t.Errorf("DefaultDeptID = %d", DefaultDeptID)
	}
	if SuperAdminRoleKey != "superadmin" {
		t.Errorf("SuperAdminRoleKey = %q", SuperAdminRoleKey)
	}

	// 状态取值须能通过 Status 正则。
	for _, s := range []string{StatusNormal, StatusDisable} {
		if !PatternStatus.MatchString(s) {
			t.Errorf("状态 %q 不满足 RegexStatus", s)
		}
	}

	if MenuTypeDir == MenuTypeMenu || MenuTypeMenu == MenuTypeButton || MenuTypeDir == MenuTypeButton {
		t.Error("菜单类型取值重复")
	}

	want := []string{"password", "oldPassword", "newPassword", "confirmPassword"}
	if len(ExcludeProperties) != len(want) {
		t.Fatalf("ExcludeProperties 长度 = %d, want %d", len(ExcludeProperties), len(want))
	}
	for i := range want {
		if ExcludeProperties[i] != want[i] {
			t.Errorf("ExcludeProperties[%d] = %q, want %q", i, ExcludeProperties[i], want[i])
		}
	}
}
