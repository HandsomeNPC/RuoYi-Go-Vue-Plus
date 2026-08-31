// Package strutil 字符串工具，对应 Java ruoyi-common-core 的 StringUtils。
package strutil

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"ruoyi-go-vue-plus/pkg/constant"
)

// Capitalize 首字母转大写，其余字符原样保留。
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	upper := unicode.ToUpper(r)
	if upper == r {
		return s
	}
	return string(upper) + s[size:]
}

// IsHTTP 判断是否 http(s) 链接。
//
// Java 侧用 hutool Validator.isUrl（能接受 ftp:// 等任意协议），这里收紧成 http/https
// 前缀判定：调用方（菜单内链）拿到 true 后会去剥 http:// 前缀拼路由，非 http 协议进来只会拼出脏路径。
func IsHTTP(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), constant.ConstantHTTP) ||
		strings.HasPrefix(strings.ToLower(link), constant.ConstantHTTPS)
}
