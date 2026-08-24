package middleware

import (
	"regexp"
	"strings"
	"testing"
)

// TestIsMatchIPRuleExact 精确相等是第一优先级。
func TestIsMatchIPRuleExact(t *testing.T) {
	if !IsMatchIPRule("192.168.1.5", "192.168.1.5") {
		t.Error("完全相同的 IP 应匹配")
	}
	if IsMatchIPRule("192.168.1.5", "192.168.1.6") {
		t.Error("不同的 IP 不应匹配")
	}
	if !IsMatchIPRule("  192.168.1.5  ", "192.168.1.5") {
		t.Error("规则两侧空白应被裁掉")
	}
}

// TestIsMatchIPRuleEmptyIsFalse 空规则或空 IP 一律 false。
func TestIsMatchIPRuleEmptyIsFalse(t *testing.T) {
	cases := [][2]string{
		{"", "192.168.1.5"},
		{"192.168.1.5", ""},
		{"", ""},
		{"   ", "192.168.1.5"},
	}
	for _, c := range cases {
		if IsMatchIPRule(c[0], c[1]) {
			t.Errorf("IsMatchIPRule(%q, %q) 应为 false", c[0], c[1])
		}
	}
}

// TestIsMatchIPRuleGlobIsFullMatch 锁住 glob 必须是全串匹配。
func TestIsMatchIPRuleGlobIsFullMatch(t *testing.T) {
	const rule = "192.168.1.*"

	// 带前缀的畸形串绝不能命中。
	for _, bad := range []string{
		"10.0.0.1#192.168.1.5",
		"x192.168.1.5",
		"a192.168.1.5z",
	} {
		if IsMatchIPRule(rule, bad) {
			t.Errorf("规则 %q 不该命中 %q（必须全串匹配）", rule, bad)
		}
	}

	for _, good := range []string{
		"192.168.1.5",
		"192.168.1.255",
		"192.168.1.",
		"192.168.1.5.evil.com",
	} {
		if !IsMatchIPRule(rule, good) {
			t.Errorf("规则 %q 应命中 %q", rule, good)
		}
	}

	// 前提校验：确认「Java 的 replace + Go 的部分匹配」真的会误判前缀。
	javaStyle := strings.NewReplacer(".", `\.`, "*", ".*", "?", ".").Replace(rule)
	if ok, err := regexp.MatchString(javaStyle, "10.0.0.1#192.168.1.5"); err != nil {
		t.Fatalf("正则编译失败: %v", err)
	} else if !ok {
		t.Error("前提已失效：照搬 Java 正则竟未误判前缀，请重新评估不用 regexp 的理由")
	}
}

// TestIsMatchIPRuleGlobQuestionMark ? 匹配单个字符。
func TestIsMatchIPRuleGlobQuestionMark(t *testing.T) {
	if !IsMatchIPRule("192.168.1.?", "192.168.1.5") {
		t.Error("? 应匹配单个字符")
	}
	if IsMatchIPRule("192.168.1.?", "192.168.1.55") {
		t.Error("? 不应匹配两个字符")
	}
}

// TestMatchCIDRv4 IPv4 网段匹配。
func TestMatchCIDRv4(t *testing.T) {
	tests := []struct {
		rule string
		ip   string
		want bool
	}{
		{"192.168.1.0/24", "192.168.1.5", true},
		{"192.168.1.0/24", "192.168.1.255", true},
		{"192.168.1.0/24", "192.168.2.1", false},
		{"10.0.0.0/8", "10.255.255.255", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"192.168.1.5/32", "192.168.1.5", true},
		{"192.168.1.5/32", "192.168.1.6", false},
		{"0.0.0.0/0", "8.8.8.8", true},
		{"192.168.1.0/33", "192.168.1.5", false},
		{"not-a-cidr/24", "192.168.1.5", false},
		{"192.168.1.0/24", "not-an-ip", false},
	}
	for _, tt := range tests {
		if got := IsMatchIPRule(tt.rule, tt.ip); got != tt.want {
			t.Errorf("IsMatchIPRule(%q, %q) = %v, want %v", tt.rule, tt.ip, got, tt.want)
		}
	}
}

// TestMatchCIDRRejectsCrossFamily 锁住 v4/v6 族别必须显式对齐。
func TestMatchCIDRRejectsCrossFamily(t *testing.T) {
	// v6 的全零网段不能命中 v4 地址。
	if IsMatchIPRule("::/0", "8.8.8.8") {
		t.Error("v6 规则 ::/0 不该命中 IPv4 地址（族别必须对齐）")
	}
	if IsMatchIPRule("2001:db8::/32", "192.168.1.5") {
		t.Error("v6 规则不该命中 IPv4 地址")
	}
	if IsMatchIPRule("0.0.0.0/0", "2001:db8::1") {
		t.Error("v4 规则 0.0.0.0/0 不该命中 IPv6 地址")
	}
	if !IsMatchIPRule("2001:db8::/32", "2001:db8::1") {
		t.Error("同族 v6 应匹配")
	}
	if !IsMatchIPRule("::1/128", "::1") {
		t.Error("v6 回环应匹配")
	}
}

// TestMatchAnyIPRule 任意一条命中即通过。
func TestMatchAnyIPRule(t *testing.T) {
	rules := []string{"10.0.0.0/8", "192.168.1.*", "172.16.0.1"}

	for _, ip := range []string{"10.1.2.3", "192.168.1.99", "172.16.0.1"} {
		if !MatchAnyIPRule(ip, rules) {
			t.Errorf("%q 应命中规则集", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "192.168.2.1", "172.16.0.2"} {
		if MatchAnyIPRule(ip, rules) {
			t.Errorf("%q 不应命中规则集", ip)
		}
	}
	if MatchAnyIPRule("8.8.8.8", nil) {
		t.Error("空规则集应返回 false")
	}
}
