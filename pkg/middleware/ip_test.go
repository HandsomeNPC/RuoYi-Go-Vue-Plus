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
	// 规则两侧的空白要裁掉，对齐 Java 的 StringUtils.trim(rule)。
	if !IsMatchIPRule("  192.168.1.5  ", "192.168.1.5") {
		t.Error("规则两侧空白应被裁掉")
	}
}

// TestIsMatchIPRuleEmptyIsFalse 空规则或空 IP 一律 false。
//
// 「没有规则」不能变成「全部放行」—— 白名单上这个默认值取反就是敞开的口子。
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

// TestIsMatchIPRuleGlobIsFullMatch 锁住 glob 必须是**全串**匹配。
//
// 这是移植 NetUtils.isMatchIpRule 时最容易踩的坑：Java 的 String.matches
// 是全串匹配，而 Go 的 regexp.MatchString 是部分匹配。照搬那三次 replace
// 再 MatchString，规则 "192.168.1.*" 会命中**带前缀**的畸形串 ——
// 白名单的判断不该依赖输入一定是合法 IP。
//
// 注意尾部方向不在此列：`*` 按 glob 语义就该匹配任意后缀，
// 所以 "192.168.1.5.evil.com" 命中 "192.168.1.*" 在 Java 侧同样成立
// （正则 `192\.168\.1\..*` 全串匹配得上）。那是通配符的语义而非移植缺陷，
// 下面 want=true 的用例把这件事一并记下来。
//
// 用例同时断言「照搬 Java 的正则 + Go 的部分匹配」确实会误判前缀，
// 后半条是为了让前提失效得明显：将来谁把实现换成正则，这里会立刻报错。
func TestIsMatchIPRuleGlobIsFullMatch(t *testing.T) {
	const rule = "192.168.1.*"

	// 带**前缀**的畸形串绝不能命中 —— 这正是部分匹配会漏掉的方向。
	for _, bad := range []string{
		"10.0.0.1#192.168.1.5",
		"x192.168.1.5",
		"a192.168.1.5z",
	} {
		if IsMatchIPRule(rule, bad) {
			t.Errorf("规则 %q 不该命中 %q（必须全串匹配，不能只匹配中间一段）", rule, bad)
		}
	}

	// 正常的段内通配要生效；尾部任意后缀按 glob 语义也算命中（与 Java 一致）。
	for _, good := range []string{
		"192.168.1.5",
		"192.168.1.255",
		"192.168.1.",
		"192.168.1.5.evil.com", // `*` 吞掉整个后缀，Java 侧同样成立
	} {
		if !IsMatchIPRule(rule, good) {
			t.Errorf("规则 %q 应命中 %q", rule, good)
		}
	}

	// 前提校验：确认「Java 的 replace + Go 的部分匹配」真的会误判前缀。
	// 若这里不再失败，说明可以考虑用正则实现了。
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
		{"0.0.0.0/0", "8.8.8.8", true}, // 全放行
		// 非法规则与非法 IP 一律不匹配，不 panic。
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
//
// Go 的 net.IP 把 IPv4 规范化成 16 字节的 v4-mapped 形式（::ffff:1.2.3.4），
// 于是 net.IPNet.Contains 对 "::/0" 这条 v6 规则会把 IPv4 地址也算进去 ——
// **一条本意只放行 IPv6 的规则会静默放行全世界**。
// Java 侧靠 networkBytes.length != currentBytes.length 挡住这件事。
func TestMatchCIDRRejectsCrossFamily(t *testing.T) {
	// 核心用例：v6 的全零网段不能命中 v4 地址。
	if IsMatchIPRule("::/0", "8.8.8.8") {
		t.Error("v6 规则 ::/0 不该命中 IPv4 地址（族别必须对齐）")
	}
	if IsMatchIPRule("2001:db8::/32", "192.168.1.5") {
		t.Error("v6 规则不该命中 IPv4 地址")
	}
	// 反向：v4 规则不该命中 v6 地址。
	if IsMatchIPRule("0.0.0.0/0", "2001:db8::1") {
		t.Error("v4 规则 0.0.0.0/0 不该命中 IPv6 地址")
	}
	// 同族仍要正常工作。
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
	// 空规则集返回 false —— 调用方在白名单为空时才跳过检查，那是另一层判断。
	if MatchAnyIPRule("8.8.8.8", nil) {
		t.Error("空规则集应返回 false")
	}
}
