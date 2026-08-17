package constant

// 常用正则表达式字符串
//
// 常用正则表达式集合，更多正则见: https://any86.github.io/any-rule/
//
// @author AprilWind
const (
	// RegexDictionaryType 字典类型必须以字母开头，且只能为（小写字母，数字，下滑线）
	RegexDictionaryType = "^[a-z][a-z0-9_]*$"

	// RegexPermissionString 权限标识必须符合以下格式：
	//  1. 标准格式：xxx:yyy:zzz
	//     - 第一部分（xxx）：只能包含字母、数字和下划线（_），不能使用 `*`
	//     - 第二部分（yyy）：可以包含字母、数字、下划线（_）和 `*`
	//     - 第三部分（zzz）：可以包含字母、数字、下划线（_）和 `*`
	//  2. 允许空字符串（""），表示没有权限标识
	RegexPermissionString = `^$|^[a-zA-Z0-9_]+:[a-zA-Z0-9_*]+:[a-zA-Z0-9_*]+$`

	// RegexIDCardLast6 身份证号码（后6位）
	RegexIDCardLast6 = `^(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$`

	// RegexQQNumber QQ号码
	RegexQQNumber = `^[1-9][0-9]\d{4,9}$`

	// RegexPostalCode 邮政编码
	RegexPostalCode = `^[1-9]\d{5}$`

	// RegexAccount 注册账号
	RegexAccount = `^[a-zA-Z][a-zA-Z0-9_]{4,15}$`

	// RegexPassword 密码：包含至少8个字符，包括大写字母、小写字母、数字和特殊字符
	RegexPassword = `^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$`

	// RegexStatus 通用状态（0表示正常，1表示停用）
	RegexStatus = `^[01]$`
)
