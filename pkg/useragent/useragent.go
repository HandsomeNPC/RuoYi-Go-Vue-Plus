// Package useragent 解析 User-Agent，对应 Java hutool UserAgentUtil。
package useragent

import (
	"strings"

	"github.com/mssola/useragent"
)

// Parse 解析 UA 字符串，返回浏览器名与操作系统名（均含版本以外的描述）。
// 解析失败返回空串。对应 Java UserAgentUtil.parse(...).getBrowser/getName。
func Parse(ua string) (browser, os string) {
	if strings.TrimSpace(ua) == "" {
		return "", ""
	}
	p := useragent.New(ua)
	if p == nil {
		return "", ""
	}
	name, _ := p.Browser()
	return strings.TrimSpace(name), strings.TrimSpace(p.OS())
}
