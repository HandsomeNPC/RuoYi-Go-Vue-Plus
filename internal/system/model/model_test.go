package model

import (
	"reflect"
	"strings"
	"testing"
)

// TestSupportsGrantTypeIsExactNotSubstring 锁住相对 Java 的一处有意偏差。
//
// Java 侧是 `StringUtils.contains(client.getGrantType(), grantType)`
// （AuthController.java:86）—— 对逗号拼接串做**子串**匹配，于是
// grantType="pass" 会命中 "password,social"。那是 bug 不是可对齐的行为：
// 一个拼错或恶意构造的 grantType 会通过校验，随后在策略分派处才失败
// （或更糟，命中了别的策略）。
//
// 用例同时断言子串匹配确实会误判，让前提失效得明显。
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

	// 前提校验：确认 Java 的子串匹配真的会误判。
	if !strings.Contains(c.GrantType, "pass") {
		t.Error("前提已失效：子串匹配竟不会误判 \"pass\"，请重新评估本偏差的必要性")
	}
}

// TestAccessPathListNormalizes 归一化必须补上前导斜杠。
//
// 对应 SysClientServiceImpl.normalizeAccessPath（:254-266）。
// 这一步不能省：Ant 匹配要求 pattern 与 path 同时以 / 开头
// （见 pkg/middleware/path.go 的 AntPathMatch），而请求路径必然以 / 开头 ——
// 规则漏了它就一条也匹配不上，表现为「配了白名单反而全被拒」。
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
//
// 对应 Java 侧对这一项传 UnaryOperator.identity()。给 IP 规则补前导斜杠
// 会让每一条都失效。
func TestIPWhitelistListDoesNotNormalize(t *testing.T) {
	c := &SysClient{IPWhitelist: "10.0.0.0/8, 192.168.1.*"}

	want := []string{"10.0.0.0/8", "192.168.1.*"}
	if got := c.IPWhitelistList(); !reflect.DeepEqual(got, want) {
		t.Errorf("IPWhitelistList() = %v, 期望 %v(不应补斜杠)", got, want)
	}
}

// TestSplitRulesSeparators 按 , ; CR LF 切分，合并连续分隔符、trim、丢空段。
//
// 对应 Java 的 CLIENT_RULE_SEPARATOR_REGEX "[,;\r\n]+" 配
// str2List(…, ignoreEmpty=true, isTrim=true)。
//
// 这份语义必须与 pkg/middleware/auth.go 的 splitClientRules 一致 ——
// 那边切 JWT claims 里的规则串，这边切 DB 实体。两处没有合并成一处
// （合并会让 pkg 依赖 internal，破坏分层），一致性靠各自的测试锁住。
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

// TestTableNames 表名必须与原项目一致 —— 写错了查的是另一张（不存在的）表。
func TestTableNames(t *testing.T) {
	if got, want := (SysUser{}).TableName(), "sys_user"; got != want {
		t.Errorf("SysUser.TableName() = %q, 期望 %q", got, want)
	}
	if got, want := (SysClient{}).TableName(), "sys_client"; got != want {
		t.Errorf("SysClient.TableName() = %q, 期望 %q", got, want)
	}
}

// TestPasswordAndSecretNeverSerialized 锁住密码与客户端密钥不会漏进响应体。
//
// SysUser.Password 与 SysClient.ClientSecret 都必须带 json:"-"，
// 对齐 Java SysUserVo 上 @JsonIgnore + @JsonProperty 的「只读入不写出」。
// 少这个标签，任何直接返回实体的接口都会把哈希/密钥泄出去 ——
// 而那不会有任何编译期或运行期症状。
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
