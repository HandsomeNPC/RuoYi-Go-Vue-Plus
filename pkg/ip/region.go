package ip

import (
	_ "embed"
	"net"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// v4Content 内嵌的 IPv4 xdb 地址库。
//
//go:embed data/ip2region_v4.xdb
var v4Content []byte

// RealAddressByIP 离线查询 IP 的行政区域。
// 仅支持 IPv4；内网 IP 返回 "内网IP"；IPv6 或查询失败返回 "未知"。
// 每次调用新建 Searcher（共享只读 buffer），避免并发写 ioCount 的竞争。
func RealAddressByIP(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return "未知"
	}
	p := net.ParseIP(ipStr)
	if p == nil {
		return "未知"
	}
	// 内网不查询
	if p.IsPrivate() || p.IsLoopback() || p.IsLinkLocalUnicast() || p.IsUnspecified() {
		return "内网IP"
	}
	v4 := p.To4()
	if v4 == nil {
		return "未知" // IPv6 暂未带库
	}
	s, err := xdb.NewWithBuffer(xdb.IPv4, v4Content)
	if err != nil {
		return "未知"
	}
	region, err := s.Search(v4.String())
	if err != nil || region == "" {
		return "未知"
	}
	return region
}
