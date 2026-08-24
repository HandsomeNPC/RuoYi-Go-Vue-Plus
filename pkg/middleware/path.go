package middleware

import "strings"

// MatchAnyPath 判断 path 是否命中 patterns 中任意一条 Ant 规则。
func MatchAnyPath(path string, patterns []string) bool {
	if path == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if AntPathMatch(p, path) {
			return true
		}
	}
	return false
}

// AntPathMatch 用单条 Ant 规则匹配路径。
func AntPathMatch(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	if strings.HasPrefix(pattern, "/") != strings.HasPrefix(path, "/") {
		return false
	}
	return matchSegments(splitPathSegments(pattern), splitPathSegments(path))
}

// splitPathSegments 按 / 切分并丢弃空段。
func splitPathSegments(s string) []string {
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchSegments 逐段匹配，`**` 可吞掉任意段数（含零段）。
func matchSegments(pat, path []string) bool {
	// memo[i][j]：pat[i:] 能否匹配 path[j:]。0=未算 1=能 2=不能。
	memo := make([][]uint8, len(pat)+1)
	for i := range memo {
		memo[i] = make([]uint8, len(path)+1)
	}

	var match func(i, j int) bool
	match = func(i, j int) bool {
		if v := memo[i][j]; v != 0 {
			return v == 1
		}
		res := matchSegmentsAt(pat, path, i, j, match)
		if res {
			memo[i][j] = 1
		} else {
			memo[i][j] = 2
		}
		return res
	}
	return match(0, 0)
}

// matchSegmentsAt 计算 pat[i:] 与 path[j:] 是否匹配，递归部分回调 match 走备忘录。
func matchSegmentsAt(pat, path []string, i, j int, match func(int, int) bool) bool {
	if i == len(pat) {
		return j == len(path)
	}

	if pat[i] == "**" {
		// 贪心不成立：只能逐个切分点试。
		for k := j; k <= len(path); k++ {
			if match(i+1, k) {
				return true
			}
		}
		return false
	}

	if j == len(path) {
		return false
	}
	if !matchSegment(pat[i], path[j]) {
		return false
	}
	return match(i+1, j+1)
}

// matchSegment 在单段内做 * / ? 通配匹配，两者都不跨 /。
func matchSegment(pat, s string) bool {
	// 绝大多数 pattern 段是字面量，直接比字符串。
	if !strings.ContainsAny(pat, "*?") {
		return pat == s
	}

	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		switch {
		case pi < len(pat) && (pat[pi] == '?' || pat[pi] == s[si]):
			pi++
			si++
		case pi < len(pat) && pat[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			// 回溯：让上一个 * 多吞一个字符。
			mark++
			pi, si = star+1, mark
		default:
			return false
		}
	}
	// path 已耗尽，pattern 剩下的必须全是 *。
	for pi < len(pat) && pat[pi] == '*' {
		pi++
	}
	return pi == len(pat)
}
