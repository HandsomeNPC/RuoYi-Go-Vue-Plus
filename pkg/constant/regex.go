package constant

import (
	"regexp"
	"strings"
)

// 常用正则，移植原项目 RegexConstants。
//
// 与原项目的两点关键差异：
//
//  1. **必须显式加锚点**。Java 的 @Pattern 是整串匹配语义，Go 的
//     regexp.MatchString 是子串搜索。原项目里自己声明的正则本就带 ^$，
//     但继承自 hutool RegexPool 的（如 MOBILE）不带，直接照搬会让
//     "abc13800138000xyz" 通过校验。这里统一补齐。
//  2. **Go 的 RE2 不支持 lookahead**。原项目 PASSWORD 用了 4 个 (?=...)，
//     regexp.MustCompile 会 panic，改用 ValidPassword 手写校验，见下。
const (
	// RegexDictionaryType 字典类型必须以字母开头，且只能为（小写字母，数字，下划线）
	RegexDictionaryType = `^[a-z][a-z0-9_]*$`

	// RegexPermissionString 权限标识格式 xxx:yyy:zzz，首段不允许 `*`，
	// 后两段允许 `*`；允许空串表示无权限标识。
	RegexPermissionString = `^$|^[a-zA-Z0-9_]+:[a-zA-Z0-9_*]+:[a-zA-Z0-9_*]+$`

	// RegexMobile 中国大陆手机号。取自 hutool RegexPool.MOBILE，
	// 原值不含锚点，这里补上 ^$ 以对齐 Java @Pattern 的整串语义。
	RegexMobile = `^(?:0|86|\+86)?1[3-9]\d{9}$`

	// RegexIDCardLast6 身份证号码（后 6 位）
	RegexIDCardLast6 = `^(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$`

	// RegexQQNumber QQ 号码
	RegexQQNumber = `^[1-9][0-9]\d{4,9}$`

	// RegexPostalCode 邮政编码
	RegexPostalCode = `^[1-9]\d{5}$`

	// RegexAccount 注册账号：字母开头，总长 5-16
	RegexAccount = `^[a-zA-Z][a-zA-Z0-9_]{4,15}$`

	// RegexStatus 通用状态（0 正常，1 停用）
	RegexStatus = `^[01]$`
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
)

// 密码校验规则，对应原项目 RegexConstants.PASSWORD：
//
//	^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$
//
// 即：长度 >= 8，只允许字母/数字/`@$!%*?&`，且四类字符各至少一个。
// RE2 无 lookahead，无法用正则表达，故拆成显式条件。
const (
	// PasswordMinLen 密码最小长度
	PasswordMinLen = 8
	// PasswordSpecialChars 密码允许的特殊字符集合
	PasswordSpecialChars = "@$!%*?&"
)

// ValidPassword 校验密码是否符合规则。
//
// 注意：原项目两处 @Pattern(PASSWORD) 都是**注释掉的**
// （PasswordLoginBody / RegisterBody），即默认不强制此规则。
// 这里只提供能力，是否启用由业务层决定，不要想当然地在登录链路上开。
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
			// 字符集之外的字符（含所有非 ASCII）一律拒绝，对齐原正则的字符类。
			return false
		}
	}
	return hasLower && hasUpper && hasDigit && hasSpecial
}
