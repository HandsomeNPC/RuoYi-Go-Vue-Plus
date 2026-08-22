package middleware

import (
	"testing"
	"time"
)

// Ant 匹配用例，行为基准是 Spring AntPathMatcher。
func TestAntPathMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		// 字面量
		{"/system/notice", "/system/notice", true},
		{"/system/notice", "/system/notices", false},
		{"/system/notice", "/system/notice/1", false},
		// 尾部斜杠与重复斜杠归一（切分时丢弃空段）
		{"/system/notice", "/system/notice/", true},
		{"/system/notice", "/system//notice", true},
		// 前导斜杠必须一致
		{"/a/b", "a/b", false},
		{"a/b", "/a/b", false},
		// * 不跨层
		{"/*.html", "/index.html", true},
		{"/*.html", "/a/index.html", false},
		{"/system/*", "/system/user", true},
		{"/system/*", "/system/user/1", false},
		// ** 跨任意层，含零层
		{"/**/*.html", "/index.html", true},
		{"/**/*.html", "/a/b/index.html", true},
		{"/**/*.html", "/a/b/index.css", false},
		{"/system/**", "/system", true},
		{"/system/**", "/system/user/1/detail", true},
		{"/**", "/anything/at/all", true},
		// ? 单字符，不跨层
		{"/a?c", "/abc", true},
		{"/a?c", "/ac", false},
		{"/a?c", "/a/c", false},
		// 混合与多个 **（回溯正确性）
		{"/**/x/**/y", "/a/x/b/y", true},
		{"/**/x/**/y", "/x/y", true},
		{"/**/x/**/y", "/a/b/y", false},
		// * 需要回溯：贪心吞到底会漏
		{"/a*b", "/axxbxxb", true},
		{"/*/api-docs", "/v3/api-docs", true},
		{"/*/api-docs/**", "/v3/api-docs/swagger-config", true},
		// 空
		{"", "", true},
		{"/**", "/", true},
	}

	for _, tc := range cases {
		if got := AntPathMatch(tc.pattern, tc.path); got != tc.want {
			t.Errorf("AntPathMatch(%q, %q) = %v, 期望 %v",
				tc.pattern, tc.path, got, tc.want)
		}
	}
}

// 空规则集必须返回 false —— 「没配排除规则」不能变成「全部排除」。
func TestMatchAnyPathEmpty(t *testing.T) {
	if MatchAnyPath("/a", nil) {
		t.Error("空规则集应返回 false")
	}
	if MatchAnyPath("", []string{"/**"}) {
		t.Error("空路径应返回 false")
	}
}

func TestMatchAnyPath(t *testing.T) {
	patterns := []string{"/system/notice", "/warm-flow/save-json"}
	for _, p := range []string{"/system/notice", "/warm-flow/save-json"} {
		if !MatchAnyPath(p, patterns) {
			t.Errorf("%q 应命中排除规则", p)
		}
	}
	for _, p := range []string{"/system/user", "/system/notice/1"} {
		if MatchAnyPath(p, patterns) {
			t.Errorf("%q 不应命中排除规则", p)
		}
	}
}

// 多个 ** 时不能退化成指数级 —— path 段数由请求方决定，
// 没有备忘录的话这就是一条用 URL 长度换 CPU 的放大路径。
func TestAntPathMatchNoExponentialBlowup(t *testing.T) {
	pattern := "/**/a/**/a/**/a/**/a/**/a/**/a/**/a/**/a/**/b"
	path := "/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a"

	done := make(chan bool, 1)
	go func() { done <- AntPathMatch(pattern, path) }()

	select {
	case got := <-done:
		if got {
			t.Error("末段 b 不存在，应不匹配")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("匹配超过 5 秒，疑似指数级回溯")
	}
}
