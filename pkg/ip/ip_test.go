package ip

import (
	"net/http/httptest"
	"testing"
)

// TestClientIPHeaderPrecedence 代理头的顺序必须对齐 ServletUtils.getClientIP。
//
// 顺序错了会让部署在多层代理后的服务取到中间代理的 IP 而非真实客户端，
// 而 IP 白名单会因此在某些环境「莫名不生效」。
func TestClientIPHeaderPrecedence(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Real-IP", "2.2.2.2")
	r.Header.Set("Proxy-Client-IP", "3.3.3.3")
	r.Header.Set("X-Forwarded-For", "1.1.1.1")

	if got, want := ClientIP(r), "1.1.1.1"; got != want {
		t.Errorf("X-Forwarded-For 优先级最高: got %q, want %q", got, want)
	}

	// 去掉最高优先级的头，应退到下一个。
	r.Header.Del("X-Forwarded-For")
	if got, want := ClientIP(r), "2.2.2.2"; got != want {
		t.Errorf("应退到 X-Real-IP: got %q, want %q", got, want)
	}
	r.Header.Del("X-Real-IP")
	if got, want := ClientIP(r), "3.3.3.3"; got != want {
		t.Errorf("应退到 Proxy-Client-IP: got %q, want %q", got, want)
	}
}

// TestClientIPTakesFirstOfForwardedChain X-Forwarded-For 是逗号分隔的链路
// （client, proxy1, proxy2），**取第一段**才是最初的客户端。
func TestClientIPTakesFirstOfForwardedChain(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2, 3.3.3.3")

	if got, want := ClientIP(r), "1.1.1.1"; got != want {
		t.Errorf("应取链路第一段: got %q, want %q", got, want)
	}
}

// TestClientIPSkipsUnknown 部分代理拿不到真实 IP 时会写 "unknown" 字面量
// 而非留空，必须跳过它继续找下一个来源。
func TestClientIPSkipsUnknown(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "unknown")
	r.Header.Set("X-Real-IP", "2.2.2.2")

	if got, want := ClientIP(r), "2.2.2.2"; got != want {
		t.Errorf("应跳过 unknown: got %q, want %q", got, want)
	}

	// 大小写不敏感，且链路里夹着 unknown 时取后面第一个有效值。
	r.Header.Set("X-Forwarded-For", "UNKNOWN, 4.4.4.4")
	if got, want := ClientIP(r), "4.4.4.4"; got != want {
		t.Errorf("应跳过 UNKNOWN 取下一段: got %q, want %q", got, want)
	}
}

// TestClientIPFallsBackToRemoteAddr 所有代理头都没有时回落到 RemoteAddr，
// 并剥掉端口 —— 相比 Java 这是必要的一步（那边 getRemoteAddr 不带端口）。
func TestClientIPFallsBackToRemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "5.5.5.5:54321"
	if got, want := ClientIP(r), "5.5.5.5"; got != want {
		t.Errorf("应回落到 RemoteAddr 并剥掉端口: got %q, want %q", got, want)
	}

	// IPv6 形如 "[::1]:8080"，剥端口后还要剥方括号。
	r.RemoteAddr = "[2001:db8::1]:8080"
	if got, want := ClientIP(r), "2001:db8::1"; got != want {
		t.Errorf("IPv6 应剥掉端口与方括号: got %q, want %q", got, want)
	}

	// 没有端口段时原样使用。
	r.RemoteAddr = "6.6.6.6"
	if got, want := ClientIP(r), "6.6.6.6"; got != want {
		t.Errorf("无端口时应原样返回: got %q, want %q", got, want)
	}
}

// TestClientIPStripsIPv6Brackets 头里的 IPv6 字面量带方括号时要剥掉，
// 对应 Java 的 StringUtils.strip(ip, "[]") —— 不剥则与白名单里的规则对不上。
func TestClientIPStripsIPv6Brackets(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "[2001:db8::1]")

	if got, want := ClientIP(r), "2001:db8::1"; got != want {
		t.Errorf("应剥掉方括号: got %q, want %q", got, want)
	}
}

// TestClientIPNilRequest 不 panic。
func TestClientIPNilRequest(t *testing.T) {
	if got := ClientIP(nil); got != "" {
		t.Errorf("nil 请求应返回空串, got %q", got)
	}
}
