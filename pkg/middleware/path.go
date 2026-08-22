package middleware

import "strings"

// Ant 风格路径匹配，对应 Java 侧 StringUtils.matches / isMatch
// （`ruoyi-common-core/…/core/utils/StringUtils.java:218,240`），底层是 Spring 的
// AntPathMatcher。三种通配语义：
//
//	?   匹配单个字符（不跨 /）
//	*   匹配一层路径内的任意字符串（不跨 /）
//	**  匹配任意层路径（可为零层）
//
// 单独抽一个文件而非塞进 xss.go：`xss.excludeUrls` 与阶段 1 的
// `security.excludes`（application.yml:100-113，含 /**/*.html 这类跨层 pattern）
// 用的是同一套语义，auth.go 会直接复用。两处各写一份的话，
// 免鉴权名单和免过滤名单迟早会在边界行为上分叉 —— 那是安全配置，不能靠巧合对齐。
//
// **有意不用 regexp**：pattern 来自配置文件（非用户输入），逐段扫描够用；
// 转正则要处理 `.` `+` `(` 这些在路径里合法、在正则里有语法意义的字符的转义，
// 漏一个就是一条静默放宽的匹配规则。

// MatchAnyPath 判断 path 是否命中 patterns 中任意一条 Ant 规则。
//
// 对应 StringUtils.matches(str, strs)：空 path 或空规则集一律返回 false ——
// 「没有配置排除规则」必须意味着「什么都不排除」，而不是「全部排除」。
// 免过滤/免鉴权名单上的这个默认值取反就是一个敞开的口子。
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
//
// 对齐 AntPathMatcher.doMatch 的两条前置约定：
//
//  1. pattern 与 path 必须**同时**以 / 开头或同时不以 / 开头，否则直接不匹配。
//  2. 按 / 切分后丢弃空段，于是 "/a/b"、"/a//b"、"/a/b/" 的段序列相同。
func AntPathMatch(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	// 对齐 doMatch 开头的 startsWith 一致性检查。
	if strings.HasPrefix(pattern, "/") != strings.HasPrefix(path, "/") {
		return false
	}
	return matchSegments(splitPathSegments(pattern), splitPathSegments(path))
}

// splitPathSegments 按 / 切分并丢弃空段，对应 tokenizeToStringArray(…, ignoreEmptyTokens=true)。
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
//
// 用 DP 备忘录而非朴素递归：`**` 的每次出现都要试遍所有切分点，
// 朴素递归在 pattern 含多个 `**` 时是指数级的 —— path 的段数由请求方决定，
// 那就成了一条用 URL 长度换 CPU 的放大路径。备忘录把它压回 O(len(pat)*len(path))。
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
		// 贪心不成立：`/**/x` 匹配 `/a/x/b/x` 要求 ** 吞到倒数第二段，
		// 而 `/**/x` 匹配 `/x` 要求它吞零段。只能逐个切分点试。
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

// matchSegment 在**单段内**做 * / ? 通配匹配，两者都不跨 /。
//
// 用带回溯的双指针而非 DP：段内长度短，且这个写法只需常量额外空间。
// star 记住最后一个 `*` 的位置，失配时让它多吞一个字符再试 ——
// 这是通配匹配的标准解法，避免了 `*` 后面还有字面量时贪心吞过头。
//
// 按 byte 比较：URL 路径里的非 ASCII 会被 percent-encode 成 ASCII，
// 段内出现裸多字节字符时 `?` 的粒度与 Java 的 char 不同，但那不是合法 URL 形态。
func matchSegment(pat, s string) bool {
	// 绝大多数 pattern 段是字面量（/system/notice 两段都没有通配符），
	// 直接比字符串，省掉逐字符扫描。
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
	// path 已耗尽，pattern 剩下的必须全是 *（能匹配空串）。
	for pi < len(pat) && pat[pi] == '*' {
		pi++
	}
	return pi == len(pat)
}
