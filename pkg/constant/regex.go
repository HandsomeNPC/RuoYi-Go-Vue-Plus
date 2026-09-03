package constant

import (
	"regexp"
	"strings"
)

// 常用正则。
const (
	// RegexDictionaryType 字典类型必须以字母开头，且只能为（小写字母，数字，下划线）。
	RegexDictionaryType = `^[a-z][a-z0-9_]*$`

	// RegexPermissionString 权限标识格式 xxx:yyy:zzz。
	RegexPermissionString = `^$|^[a-zA-Z0-9_]+:[a-zA-Z0-9_*]+:[a-zA-Z0-9_*]+$`

	// RegexMobile 中国大陆手机号。
	RegexMobile = `^(?:0|86|\+86)?1[3-9]\d{9}$`

	// RegexIDCardLast6 身份证号码（后 6 位）。
	RegexIDCardLast6 = `^(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$`

	// RegexQQNumber QQ 号码。
	RegexQQNumber = `^[1-9][0-9]\d{4,9}$`

	// RegexPostalCode 邮政编码。
	RegexPostalCode = `^[1-9]\d{5}$`

	// RegexAccount 注册账号：字母开头，总长 5-16。
	RegexAccount = `^[a-zA-Z][a-zA-Z0-9_]{4,15}$`

	// RegexStatus 通用状态（0 正常，1 停用）。
	RegexStatus = `^[01]$`

	// RegexEmail 邮箱地址，对照 hutool Validator.isEmail 所用的模式。
	// 本地部分放行常见的点/加号/连字符等，域名部分要求至少两级且顶级域为字母。
	RegexEmail = `^[\w!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[\w!#$%&'*+/=?^_` + "`" +
		`{|}~-]+)*@(?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}$`
)

var (
	PatternDictionaryType   = regexp.MustCompile(RegexDictionaryType)
	PatternPermissionString = regexp.MustCompile(RegexPermissionString)
	PatternMobile           = regexp.MustCompile(RegexMobile)
	PatternIDCardLast6      = regexp.MustCompile(RegexIDCardLast6)
	PatternQQNumber         = regexp.MustCompile(RegexQQNumber)
	PatternPostalCode       = regexp.MustCompile(RegexPostalCode)
	PatternAccount          = regexp.MustCompile(RegexAccount)
	PatternStatus           = regexp.MustCompile(RegexStatus)
	PatternEmail            = regexp.MustCompile(RegexEmail)
)

// 密码校验参数。
const (
	// PasswordMinLen 密码最小长度。
	PasswordMinLen = 8
	// PasswordSpecialChars 密码允许的特殊字符集合。
	PasswordSpecialChars = "@$!%*?&"
)

// ValidPassword 校验密码是否符合规则。
func ValidPassword(s string) bool {
	if len(s) < PasswordMinLen {
		return false
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= '0' && c <= '9':
			hasDigit = true
		case strings.ContainsRune(PasswordSpecialChars, c):
			hasSpecial = true
		default:
			return false
		}
	}
	return hasLower && hasUpper && hasDigit && hasSpecial
}
