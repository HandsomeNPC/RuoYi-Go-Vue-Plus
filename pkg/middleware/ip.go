package middleware

import (
	"net"
	"net/http"
	"strings"
)

// 客户端 IP 与 IP 规则匹配，对应原项目
// ruoyi-common-core/…/core/utils/NetUtils.java:93-149（isMatchIpRule / isMatchCidr）
// 与 core/utils/ServletUtils.java:277-285（getClientIP）。
//
// 单独抽一个文件而非塞进 auth.go，理由与 path.go 相同：它是**匹配原语**，
// 与「谁在用它」无关。当前只有客户端 IP 白名单用到，阶段 4+ 的
// @RateLimiter（按 IP 限流）会复用同一套取 IP 的逻辑 ——
// 两处各写一份的话，「这个请求的客户端 IP 是什么」会有两个答案。

// clientIPHeaders 依次尝试的代理头，顺序对齐 ServletUtils.getClientIP。
//
// 顺序不能改：X-Forwarded-For 在最前，因为标准反代（nginx）写的是它；
// 后面几个是各家中间件的历史遗留（WebLogic、Apache mod_proxy）。
// 取到第一个非空且不为 unknown 的就停 —— 与 hutool getClientIPByHeader 一致。
var clientIPHeaders = []string{
	"X-Forwarded-For",
	"X-Real-IP",
	"Proxy-Client-IP",
	"WL-Proxy-Client-IP",
	"HTTP_CLIENT_IP",
	"HTTP_X_FORWARDED_FOR",
}

// ClientIP 取请求的客户端 IP。
//
// **这些头都是可伪造的**：任何客户端都能自己发一个 X-Forwarded-For。
// 之所以仍然信任它们，是因为本项目部署在 nginx 之后，由 nginx 覆写该头
// （deploy/nginx.conf）。进程直接暴露公网时，IP 白名单这类基于它的判断
// 就形同虚设 —— 这一点与原项目相同，不是本实现引入的。
//
// 相比 Java 多一步回落到 RemoteAddr：那边 getClientIPByHeader 取不到时
// 返回 request.getRemoteAddr()，Go 的 http.Request 对应字段是
// RemoteAddr（形如 "1.2.3.4:5678"），需要自己剥掉端口。
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	for _, h := range clientIPHeaders {
		// X-Forwarded-For 按 RFC 7239 是逗号分隔的链路（client, proxy1, proxy2），
		// **取第一段**才是最初的客户端 —— 对齐 hutool 的 split(",")[0]。
		if ip := firstValidIP(r.Header.Get(h)); ip != "" {
			return ip
		}
	}

	// RemoteAddr 带端口，且 IPv6 形如 "[::1]:8080"。
	// SplitHostPort 失败说明没有端口段，原样使用。
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return stripIPBrackets(host)
	}
	return stripIPBrackets(r.RemoteAddr)
}

// firstValidIP 从逗号分隔的头值里取第一个有效 IP，全部无效时返回空串。
//
// 跳过 "unknown"（大小写不敏感）是对齐 hutool：部分代理在拿不到真实 IP 时
// 会写这个字面量而非留空。
func firstValidIP(value string) string {
	if value == "" {
		return ""
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "unknown") {
			continue
		}
		return stripIPBrackets(part)
	}
	return ""
}

// stripIPBrackets 剥掉 IPv6 字面量的方括号，对应 Java 的 StringUtils.strip(ip, "[]")。
func stripIPBrackets(ip string) string {
	return strings.Trim(strings.TrimSpace(ip), "[]")
}

// IsMatchIPRule 判断客户端 IP 是否命中单条规则，对应 NetUtils.isMatchIpRule。
//
// 优先级照抄原实现，顺序有意义：
//
//  1. 精确相等（字符串比较，不解析）
//  2. 含 "/" → 按 CIDR 匹配
//  3. 含 "*" 或 "?" → 按 glob 匹配
//  4. 其余一律 false
//
// 空规则或空 IP 返回 false —— 「没有规则」不能变成「全部放行」。
// 调用方（auth.go）在整个白名单为空时才跳过检查，那是另一层判断。
func IsMatchIPRule(rule, clientIP string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" || clientIP == "" {
		return false
	}

	if rule == clientIP {
		return true
	}
	if strings.Contains(rule, "/") {
		return matchCIDR(rule, clientIP)
	}
	if strings.ContainsAny(rule, "*?") {
		return matchIPGlob(rule, clientIP)
	}
	return false
}

// MatchAnyIPRule 判断客户端 IP 是否命中任意一条规则。
//
// 对应 SecurityConfig.validateClientAccessRules 里的
// ipWhitelistList.stream().anyMatch(rule -> NetUtils.isMatchIpRule(rule, clientIp))。
func MatchAnyIPRule(clientIP string, rules []string) bool {
	for _, rule := range rules {
		if IsMatchIPRule(rule, clientIP) {
			return true
		}
	}
	return false
}

// matchCIDR 按 CIDR 网段匹配，对应 NetUtils.isMatchCidr。
//
// **必须显式对齐地址族**。Java 侧靠 `networkBytes.length != currentBytes.length`
// 挡住 v4/v6 混比；Go 的 net.IP 会把 IPv4 规范化成 16 字节的
// v4-mapped 形式（::ffff:1.2.3.4），于是 net.IPNet.Contains 对
// "::/0" 这条 v6 规则会把 IPv4 地址也算进去 —— 一条本意只放行 IPv6 的规则
// 会静默放行全世界。所以先各自 To4()，一方是 v4 另一方不是就直接不匹配。
func matchCIDR(rule, clientIP string) bool {
	_, network, err := net.ParseCIDR(rule)
	if err != nil {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// To4() 非 nil 即为 IPv4（含 v4-mapped 形式）。两边必须同族。
	if (network.IP.To4() != nil) != (ip.To4() != nil) {
		return false
	}
	return network.Contains(ip)
}

// matchIPGlob 按通配符匹配，对应 NetUtils.isMatchIpRule 里的 glob 分支。
//
// Java 的实现是把规则转成正则再 `clientIp.matches(regex)`：
//
//	rule.replace(".", "\\.").replace("*", ".*").replace("?", ".")
//
// **Go 侧改为逐字符匹配，不转正则** —— 两个理由：
//
//  1. Java 的 String.matches 是**全串**匹配，Go 的 regexp.MatchString 是
//     部分匹配。照搬那三次 replace 再 MatchString，规则 "192.168.1.*"
//     会命中 "10.0.0.1#192.168.1.5"（虽然那不是合法 IP，但白名单的判断
//     不该依赖输入一定合法）。要修就得自己补 ^$ 锚点 —— 一个容易漏、
//     漏了就是静默放宽的口子。
//  2. Java 那三次 replace 只转义了 `.`，IPv6 规则里的 `:` 在正则里无特殊
//     含义还算走运，但这种「恰好没问题」的转义不值得复刻。
//
// 复用 path.go 的 matchSegment：那是同一套 `*`/`?` 语义的带回溯双指针实现，
// 且已被 path_test.go 覆盖。唯一的语义差别是 path.go 里 `*` 不跨 `/`，
// 而 IP 里没有 `/`（有 `/` 的走 CIDR 分支），故两者在此等价。
func matchIPGlob(rule, clientIP string) bool {
	return matchSegment(rule, clientIP)
}
