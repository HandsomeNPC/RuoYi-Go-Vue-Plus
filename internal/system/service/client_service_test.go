package service

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/vo"
)

// TestFillRuleFieldsAccessPath 归一化必须补上前导斜杠。
func TestFillRuleFieldsAccessPath(t *testing.T) {
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
			c := &vo.SysClientVo{AccessPath: tt.in}
			ClientSvcApp.fillRuleFields(c)
			if got := c.AccessPathList; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AccessPathList = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestFillRuleFieldsIPWhitelist IP 规则原样保留，不做路径归一化。
func TestFillRuleFieldsIPWhitelist(t *testing.T) {
	c := &vo.SysClientVo{IPWhitelist: "10.0.0.0/8, 192.168.1.*"}
	ClientSvcApp.fillRuleFields(c)

	want := []string{"10.0.0.0/8", "192.168.1.*"}
	if got := c.IPWhitelistList; !reflect.DeepEqual(got, want) {
		t.Errorf("IPWhitelistList = %v, 期望 %v(不应补斜杠)", got, want)
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

// TestToVoList 列表转换须逐行回填规则字段，否则前端拿不到 *List。
func TestToVoList(t *testing.T) {
	// 取自种子数据的两个客户端。
	got := ClientSvcApp.toVoList([]*model.SysClient{
		{ClientKey: "pc", GrantType: "password,social"},
		{ClientKey: "app", GrantType: "password,sms,social", AccessPath: "app/**"},
	})

	if len(got) != 2 {
		t.Fatalf("转换后条数 = %d, 期望 2", len(got))
	}
	if want := []string{"password", "social"}; !reflect.DeepEqual(got[0].GrantTypeList, want) {
		t.Errorf("GrantTypeList = %v, 期望 %v", got[0].GrantTypeList, want)
	}
	// 归一化须补上前导斜杠。
	if want := []string{"/app/**"}; !reflect.DeepEqual(got[1].AccessPathList, want) {
		t.Errorf("AccessPathList = %v, 期望 %v", got[1].AccessPathList, want)
	}
	// 种子数据 ip_whitelist 为 NULL，切分后应为 nil。
	if got[0].IPWhitelistList != nil {
		t.Errorf("IPWhitelistList = %v, 期望 nil", got[0].IPWhitelistList)
	}
	// 原始串须原样保留，前端编辑态要用。
	if got[0].GrantType != "password,social" {
		t.Errorf("GrantType = %q, 不应被改写", got[0].GrantType)
	}
}

// TestToVoListNil 空入参不得 panic；返回 nil 由 pkgrepo.Page 兜成 []。
func TestToVoListNil(t *testing.T) {
	if got := ClientSvcApp.toVoList(nil); got != nil {
		t.Errorf("toVoList(nil) = %v, 期望 nil", got)
	}
}

// TestResolveRuleValue raw 非空时切分 raw，否则用 list；list 元素只 trim 不再切分。
func TestResolveRuleValue(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		list      []string
		normalize func(string) string
		want      string
	}{
		{"两者都空", "", nil, nil, ""},
		{"raw 优先于 list", "a,b", []string{"ignored"}, nil, "a,b"},
		{"raw 为空回落 list", "", []string{"a", "b"}, nil, "a,b"},
		{"raw 多分隔符切分", "a;b\nc", nil, nil, "a,b,c"},
		{"raw 去空段与空白", " a , , b ", nil, nil, "a,b"},
		{"list 元素只 trim", "", []string{" a ", "b"}, nil, "a,b"},
		{"list 丢弃空元素", "", []string{"a", "", "  "}, nil, "a"},
		// 归一化器作用于单条规则，raw/list 两条路径都要过。
		{"raw 走归一化", "app/**", nil, normalizeAccessPath, "/app/**"},
		{"list 走归一化", "", []string{"app/**", "*"}, normalizeAccessPath, "/app/**,/**"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRuleValue(tt.raw, tt.list, tt.normalize); got != tt.want {
				t.Errorf("resolveRuleValue(%q, %v) = %q, 期望 %q",
					tt.raw, tt.list, got, tt.want)
			}
		})
	}
}

// TestResolveRuleValueRoundTrip 入库串再切回列表须与入参等价，保证 fillRuleFields 能还原。
func TestResolveRuleValueRoundTrip(t *testing.T) {
	stored := resolveRuleValue("", []string{"app/**", "*"}, normalizeAccessPath)

	want := []string{"/app/**", "/**"}
	if got := parseAccessPathList(stored); !reflect.DeepEqual(got, want) {
		t.Errorf("落库 %q 回读 = %v, 期望 %v", stored, got, want)
	}
}

// TestNewClientID client_id 取 md5(clientKey + clientSecret)，对齐 Java SecureUtil.md5。
func TestNewClientID(t *testing.T) {
	// 与 Java 同算法的已知取值，改动实现会在此暴露。
	if got, want := newClientID("pc", "pc123"), "2ce0a4f4bf6c4a854a97a5ddfd941ebb"; got != want {
		t.Errorf("newClientID(pc, pc123) = %q, 期望 %q", got, want)
	}
	if got, want := newClientID("app", "app123"), "819aaa7e32af91de5e7dcae60fb77e10"; got != want {
		t.Errorf("newClientID(app, app123) = %q, 期望 %q", got, want)
	}
	// 32 位小写十六进制，前端按字符串比对。
	if got := newClientID("k", "s"); len(got) != 32 {
		t.Errorf("newClientID 长度 = %d, 期望 32", len(got))
	}
	// 注意 md5 作用于拼接后的整串，故 (ab,c) 与 (a,bc) 必然同值——
	// 这是 Java 的既有行为，不是缺陷，此处不做区分性断言。
}
