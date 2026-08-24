// Package ip 提供从 HTTP 请求中识别客户端 IP 的工具。
//
// 对应原项目 ruoyi-common-core/…/core/utils/ServletUtils.java:277-285
// （getClientIP）。
//
// 单独成包而非塞进 pkg/middleware/auth.go：它是**取值原语**，与「谁在用它」
// 无关。当前有鉴权中间件的 IP 白名单（pkg/middleware/auth.go）和登录 handler
// 记录登录 IP 两处用到，阶段 4+ 的 @RateLimiter（按 IP 限流）会复用同一套
// 取 IP 的逻辑 —— 三处各写一份的话，「这个请求的客户端 IP 是什么」会有
// 三个答案。
package ip

import (
	"net"
	"net/http"
	"strings"
)

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
