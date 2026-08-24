// Package ip 提供从 HTTP 请求中识别客户端 IP 的工具。
package ip

import (
	"net"
	"net/http"
	"strings"
)

// clientIPHeaders 依次尝试的代理头。取到第一个非空且不为 unknown 的就停。
var clientIPHeaders = []string{
	"X-Forwarded-For",
	"X-Real-IP",
	"Proxy-Client-IP",
	"WL-Proxy-Client-IP",
	"HTTP_CLIENT_IP",
	"HTTP_X_FORWARDED_FOR",
}

// ClientIP 取请求的客户端 IP。这些代理头都是可伪造的，依赖 nginx 覆写。
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	for _, h := range clientIPHeaders {
		// X-Forwarded-For 是逗号分隔的链路，取第一段才是最初的客户端。
		if ip := firstValidIP(r.Header.Get(h)); ip != "" {
			return ip
		}
	}

	// RemoteAddr 带端口，且 IPv6 形如 "[::1]:8080"。
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return stripIPBrackets(host)
	}
	return stripIPBrackets(r.RemoteAddr)
}

// firstValidIP 从逗号分隔的头值里取第一个有效 IP，全部无效时返回空串。
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

// stripIPBrackets 剥掉 IPv6 字面量的方括号。
func stripIPBrackets(ip string) string {
	return strings.Trim(strings.TrimSpace(ip), "[]")
}
