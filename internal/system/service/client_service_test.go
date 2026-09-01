package service

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
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

// TestBuildClientUpdateColumnsAlwaysWritten 五个可编辑列无条件写入，空串即清空。
// client_id 仅在前端回填非空时才落入 SET——对齐 Java updateByBo 对 null 字段的跳过语义。
func TestBuildClientUpdateColumnsAlwaysWritten(t *testing.T) {
	// 全部规则字段留空：模拟前端清空访问路径与 IP 白名单。
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID:           1762000000000000001,
		ClientKey:    "pc",
		ClientSecret: "pc123",
	})

	for _, col := range []string{"client_key", "client_secret", "grant_type",
		"device_type", "access_path", "ip_whitelist"} {
		if _, ok := got[col]; !ok {
			t.Errorf("列 %s 缺失，须无条件写入(否则清空操作会丢失)", col)
		}
	}
	if got["access_path"] != "" || got["ip_whitelist"] != "" {
		t.Errorf("access_path/ip_whitelist = %v/%v, 期望空串(清空语义)",
			got["access_path"], got["ip_whitelist"])
	}
	// ClientID 未回填时不写 client_id，避免把既有值刷成空串。
	if _, ok := got["client_id"]; ok {
		t.Errorf("ClientID 为空时不应写入 client_id 列，got %v", got["client_id"])
	}
}

// TestBuildClientUpdateColumnsSkipsUnset status 与两个超时缺省时不得进 SET，
// 否则漏传字段会把线上 status 刷成空串、超时刷成 0 令 token 立刻失效。
func TestBuildClientUpdateColumnsSkipsUnset(t *testing.T) {
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1762000000000000001, ClientKey: "pc", ClientSecret: "pc123",
	})

	for _, col := range []string{"status", "active_timeout", "timeout"} {
		if _, ok := got[col]; ok {
			t.Errorf("列 %s 未提供时不应写入, got %v", col, got[col])
		}
	}
}

// TestBuildClientUpdateColumnsIncludesSet status 与超时给值时须落入 SET。
func TestBuildClientUpdateColumnsIncludesSet(t *testing.T) {
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1762000000000000001, ClientKey: "pc", ClientSecret: "pc123",
		Status: "1", ActiveTimeout: 1800, Timeout: 604800,
	})

	if got["status"] != "1" {
		t.Errorf("status = %v, 期望 \"1\"", got["status"])
	}
	if got["active_timeout"] != int64(1800) || got["timeout"] != int64(604800) {
		t.Errorf("active_timeout/timeout = %v/%v, 期望 1800/604800",
			got["active_timeout"], got["timeout"])
	}
	// status="0"(正常)是有效值，不能被当成缺省丢掉。
	zero := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1, ClientKey: "k", ClientSecret: "s", Status: "0",
	})
	if zero["status"] != "0" {
		t.Errorf("status=\"0\" 应写入, got %v", zero["status"])
	}
}

// TestBuildClientUpdateColumnsClientID client_id 直接采用前端回填值，对齐 Java updateByBo
// @CacheEvict(key = "#bo.clientId") 的语义——evict 的是前端回传的那个 key，
// 服务端不重算（前端在修改 key/secret 时自行更新 clientId 回传）。
func TestBuildClientUpdateColumnsClientID(t *testing.T) {
	// 前端回填了 clientId：原样落库。
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1762000000000000001, ClientKey: "pc", ClientSecret: "pc123",
		ClientID: "2ce0a4f4bf6c4a854a97a5ddfd941ebb",
	})
	if got["client_id"] != "2ce0a4f4bf6c4a854a97a5ddfd941ebb" {
		t.Errorf("client_id = %v, 期望原样落库前端回填值", got["client_id"])
	}

	// 前端未回填 clientId（空串）：不写入，保留库里现有值。
	empty := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1762000000000000001, ClientKey: "pc", ClientSecret: "pc123",
	})
	if _, ok := empty["client_id"]; ok {
		t.Errorf("ClientID 为空时不应写入 client_id 列，got %v", empty["client_id"])
	}
}

// TestBuildClientUpdateColumnsGrantTypeJoin 授权类型组只拼接，与新增路径一致。
func TestBuildClientUpdateColumnsGrantTypeJoin(t *testing.T) {
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1, ClientKey: "k", ClientSecret: "s",
		GrantTypeList: []string{"password", "sms", "social"},
	})

	if want := "password,sms,social"; got["grant_type"] != want {
		t.Errorf("grant_type = %v, 期望 %q", got["grant_type"], want)
	}
	// 清空授权类型须落成空串，而非从 SET 里消失。
	empty := buildClientUpdateColumns(&bo.SysClientBo{ID: 1, ClientKey: "k", ClientSecret: "s"})
	if empty["grant_type"] != "" {
		t.Errorf("grant_type = %v, 期望空串", empty["grant_type"])
	}
}

// TestBuildClientUpdateColumnsNormalizesAccessPath 访问路径入库前须补前导斜杠，
// 与回读时的 fillRuleFields 归一化保持一致。
func TestBuildClientUpdateColumnsNormalizesAccessPath(t *testing.T) {
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1, ClientKey: "k", ClientSecret: "s",
		AccessPathList:  []string{"app/**", "*"},
		IPWhitelistList: []string{"10.0.0.0/8"},
	})

	if want := "/app/**,/**"; got["access_path"] != want {
		t.Errorf("access_path = %v, 期望 %q", got["access_path"], want)
	}
	// IP 白名单不走路径归一化，不该被补斜杠。
	if want := "10.0.0.0/8"; got["ip_whitelist"] != want {
		t.Errorf("ip_whitelist = %v, 期望 %q(不补斜杠)", got["ip_whitelist"], want)
	}
}

// TestBuildClientUpdateColumnsOmitsImmutable 主键与审计列不得出现在 SET 里：
// id 是定位条件，update_by/update_time 由 pkg/repository 的回调补齐。
func TestBuildClientUpdateColumnsOmitsImmutable(t *testing.T) {
	got := buildClientUpdateColumns(&bo.SysClientBo{
		ID: 1762000000000000001, ClientKey: "k", ClientSecret: "s",
	})

	for _, col := range []string{"id", "del_flag", "create_by", "create_time",
		"create_dept", "update_by", "update_time"} {
		if _, ok := got[col]; ok {
			t.Errorf("列 %s 不该由 service 写入", col)
		}
	}
}
