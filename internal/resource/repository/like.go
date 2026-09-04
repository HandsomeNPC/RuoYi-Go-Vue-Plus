package repository

import "strings"

// likeEscaper 转义 LIKE 模式里的元字符，使入参按字面量匹配。
// 不转义的话搜 "%" 会命中全表、"_" 会变成任意单字符通配——
// 这是与 likeIfText（不转义）的有意差异。
// 反斜杠必须排在最前：否则会把后两条刚补上的转义符再转一次。
//
// 与 internal/system/repository 的同名实现重复而非共用：那边是 system 包私有，
// 跨包取不到，且这几行没必要为它单开一个公共包。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLike 见 likeEscaper。
func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}
