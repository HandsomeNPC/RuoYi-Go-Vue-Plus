package constant

import "regexp"

// RegexDictionaryType 字典类型必须以字母开头，且只能为（小写字母，数字，下划线）。
const RegexDictionaryType = `^[a-z][a-z0-9_]*$`

// PatternDictionaryType 字典类型校验。
var PatternDictionaryType = regexp.MustCompile(RegexDictionaryType)
