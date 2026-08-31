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
