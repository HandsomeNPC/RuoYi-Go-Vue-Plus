package repository

import "strings"

// likeEscaper 转义 LIKE 模式里的元字符，使入参按字面量匹配。
// 不转义的话搜 "%" 会命中全表、"_" 会变成任意单字符通配。
// 反斜杠必须排在最前：否则会把后两条刚补上的转义符再转一次。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike 转义 LIKE 模式里的元字符，返回可安全拼进 LIKE 模式的字面量。
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
