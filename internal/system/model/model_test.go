package model

import (
	"reflect"
	"strings"
	"testing"
)

// TestSupportsGrantTypeIsExactNotSubstring 锁住授权类型精确比对。
func TestSupportsGrantTypeIsExactNotSubstring(t *testing.T) {
	c := &SysClient{GrantType: "password,social"}

	for _, ok := range []string{"password", "social"} {
		if !c.SupportsGrantType(ok) {
			t.Errorf("应支持 %q", ok)
		}
	}

	// 子串但非完整取值，必须拒绝。
	for _, bad := range []string{"pass", "word", "soc", "ord", "sms", ""} {
		if c.SupportsGrantType(bad) {
			t.Errorf("%q 不是完整的授权类型，不该通过", bad)
		}
	}

	// 前提校验：确认子串匹配真的会误判。
	if !strings.Contains(c.GrantType, "pass") {
		t.Error("前提已失效：子串匹配竟不会误判 \"pass\"，请重新评估本偏差的必要性")
	}
}

// TestAccessPathListNormalizes 归一化必须补上前导斜杠。
func TestAccessPathListNormalizes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"空", "", nil},
		{"补前导斜杠", "app/**", []string{"/app/**"}},
		{"已有斜杠保持原样", "/app/**", []string{"/app/**"}},
		{"星号归一成全放行", "*", []string{"/**"}},
		{"双星保持全放行", "/**", []string{"/**"}},
		{"多条混合", "app/**,/pub/**", []string{"/app/**", "/pub/**"}},
		// 种子数据里 app 客户端就是 /app/**。
		{"种子数据", "/app/**", []string{"/app/**"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &SysClient{AccessPath: tt.in}
			if got := c.AccessPathList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AccessPathList() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestIPWhitelistListDoesNotNormalize IP 规则原样保留，不做路径归一化。
func TestIPWhitelistListDoesNotNormalize(t *testing.T) {
	c := &SysClient{IPWhitelist: "10.0.0.0/8, 192.168.1.*"}

	want := []string{"10.0.0.0/8", "192.168.1.*"}
	if got := c.IPWhitelistList(); !reflect.DeepEqual(got, want) {
		t.Errorf("IPWhitelistList() = %v, 期望 %v(不应补斜杠)", got, want)
	}
}

// TestSplitRulesSeparators 按 , ; CR LF 切分，合并连续分隔符、trim、丢空段。
func TestSplitRulesSeparators(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a;b", []string{"a", "b"}},
		{"a\nb", []string{"a", "b"}},
		{"a\r\nb", []string{"a", "b"}},
		{"a,,;b", []string{"a", "b"}},   // 连续分隔符合并
		{" a , b ", []string{"a", "b"}}, // trim
		{",,,", nil},                    // 全是分隔符
		{"  ", nil},                     // 全是空白
	}
	for _, tt := range tests {
		if got := splitRules(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitRules(%q) = %v, 期望 %v", tt.in, got, tt.want)
		}
	}
}

// TestTableNames 表名必须与原项目一致。
func TestTableNames(t *testing.T) {
	if got, want := (SysUser{}).TableName(), "sys_user"; got != want {
		t.Errorf("SysUser.TableName() = %q, 期望 %q", got, want)
	}
	if got, want := (SysClient{}).TableName(), "sys_client"; got != want {
		t.Errorf("SysClient.TableName() = %q, 期望 %q", got, want)
	}
}

// TestPasswordAndSecretNeverSerialized 锁住密码与客户端密钥不会漏进响应体。
func TestPasswordAndSecretNeverSerialized(t *testing.T) {
	userField, ok := reflect.TypeOf(SysUser{}).FieldByName("Password")
	if !ok {
		t.Fatal("SysUser 没有 Password 字段")
	}
	if got := userField.Tag.Get("json"); got != "-" {
		t.Errorf(`SysUser.Password 的 json 标签 = %q, 必须为 "-"(否则密码哈希会漏进响应体)`, got)
	}

	clientField, ok := reflect.TypeOf(SysClient{}).FieldByName("ClientSecret")
	if !ok {
		t.Fatal("SysClient 没有 ClientSecret 字段")
	}
	if got := clientField.Tag.Get("json"); got != "-" {
		t.Errorf(`SysClient.ClientSecret 的 json 标签 = %q, 必须为 "-"`, got)
	}
}
