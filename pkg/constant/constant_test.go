package constant

import (
	"strings"
	"testing"
	"time"
)

// 正则必须整串匹配。Java 的 @Pattern 是整串语义，Go 的 MatchString 是子串搜索，
// 这里重点验证「前后带垃圾字符」不会误通过——这是移植时最容易踩的坑。
//
// Mobile 尤其危险：它继承自 hutool RegexPool，原值不带 ^$。
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
			// ^(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$ —— 恰好 6 位：
			// 前 2 位是「日」(01-31，但 [0-2][1-9] 排除了 x0，故 10/20/30/31 单列)，
			// 中间 3 位顺序码，末位校验位。
			name:  "IDCardLast6",
			valid: []string{"01123X", "31012x", "291234", "101234"},
			bad:   []string{"", "40123X", "00123X", "32123X", "0112X", "01123XY", "a01123X"},
			match: PatternIDCardLast6.MatchString,
		},
		{
			// ^[1-9][0-9]\d{4,9}$ —— 总长 6-11 位，首位非 0。
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

// 密码规则用手写函数而非正则（RE2 无 lookahead），语义须与原项目
// PASSWORD 正则一致：>=8 位，四类字符各至少一个，且不含字符集外的字符。
func TestValidPassword(t *testing.T) {
	valid := []string{"Abcdef1!", "aB3$aB3$", "P@ssw0rdP@ssw0rd"}
	bad := []string{
		"",          // 空
		"Abcd1!a",   // 7 位，长度不足
		"abcdef1!",  // 缺大写
		"ABCDEF1!",  // 缺小写
		"Abcdefg!",  // 缺数字
		"Abcdefg1",  // 缺特殊字符
		"Abcdef1!#", // '#' 不在允许集合内
		"Abcdef1!中", // 非 ASCII
		"Abcdef1! ", // 空格
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

// 全局 key 都必须在 global: 命名空间下，且前缀与原项目逐字一致。
//
// 这些是 const 前缀而非函数：原项目同样是字符串拼接
// （RedisUtils.setCacheObject(CAPTCHA_CODE_KEY + uuid, ...)）。
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

// 独立 key 前缀（非缓存组），拼接后须与原项目 Redis 中的 key 一致。
func TestStandaloneKeyPrefixes(t *testing.T) {
	if got, want := OnlineTokenKeyPrefix+"tk", "online_tokens:tk"; got != want {
		t.Errorf("在线用户 key = %q, want %q", got, want)
	}
	if got, want := PwdErrCntKeyPrefix+"admin", "pwd_err_cnt:admin"; got != want {
		t.Errorf("密码错误次数 key = %q, want %q", got, want)
	}
}

// 缓存组名必须是原项目 '#' 之前的那一段。
//
// 原声明形如 "sys_client#30d"，其中 "#30d" 只用于构造 CacheConfig，
// 传给 Redisson getMap() 的仅 array[0]。若把整串当组名用，会写出一个
// 名为 `sys_client#30d` 的 Hash，与原项目数据对不上 —— 这里锚死取值。
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

// TTL 逐组核对原项目 '#' 后的第一段参数。
//
// demo:cache 是唯一不走 30d 的组（原声明 "demo:cache#60s#10m#20"），
// 单独列出来防止被顺手改成 30 天。
func TestCacheTTL(t *testing.T) {
	ttls := map[string]struct {
		got  time.Duration
		want time.Duration
	}{
		"demo:cache":         {CacheTTLDemoCache, 60 * time.Second},
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

// 系统常量里被前端/DB 依赖的取值，改动即为破坏性变更。
// 超管 ID 是硬编码的雪花 ID，与原项目 SQL 初始数据强绑定，必须逐字一致。
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

	// 状态取值须能通过 Status 正则，两者同源。
	for _, s := range []string{StatusNormal, StatusDisable} {
		if !PatternStatus.MatchString(s) {
			t.Errorf("状态 %q 不满足 RegexStatus", s)
		}
	}

	// 菜单类型三值互不相同且与组件标识不冲突。
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
