package sqlcond

import (
	"strings"
	"testing"
)

// matchCSV 按 CSVContainsInt 生成的 4 个 LIKE/= 模式在内存里判定，
// 用于在不连库的情况下验证语义。模式里的 % 对应"任意字符"。
func matchCSV(csv string, cond string, args []any) bool {
	// 第 1 个参数是等值，其余 3 个是 LIKE 模式。
	for i, a := range args {
		s := a.(string)
		if i == 0 {
			if csv == s {
				return true
			}
			continue
		}
		if likeMatch(csv, s) {
			return true
		}
	}
	return false
}

// likeMatch 实现仅含前后 % 的简化 LIKE。
func likeMatch(s, pat string) bool {
	hasPrefix := strings.HasPrefix(pat, "%")
	hasSuffix := strings.HasSuffix(pat, "%")
	core := strings.Trim(pat, "%")
	switch {
	case hasPrefix && hasSuffix:
		return strings.Contains(s, core)
	case hasPrefix:
		return strings.HasSuffix(s, core)
	case hasSuffix:
		return strings.HasPrefix(s, core)
	default:
		return s == core
	}
}

// TestCSVContainsInt_整段匹配 覆盖四种位置，以及最关键的「前缀不得误命中」。
func TestCSVContainsInt_整段匹配(t *testing.T) {
	cases := []struct {
		name string
		csv  string
		v    int64
		want bool
	}{
		{"独占整串", "100", 100, true},
		{"串首", "100,200,300", 100, true},
		{"串尾", "1,2,100", 100, true},
		{"串中", "1,100,2", 100, true},

		// 这组是本函数存在的理由：子串匹配会在这里越权。
		{"前缀不得命中", "1001,1002", 100, false},
		{"后缀不得命中", "2100,3100", 100, false},
		{"包含不得命中", "11007", 100, false},
		{"空串", "", 100, false},
		{"他值", "200,300", 100, false},

		// 顶级部门的祖先链形如 "0"，不该命中真实部门 ID。
		{"零值串", "0", 100, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cond, args := CSVContainsInt("ancestors", c.v)
			if !strings.Contains(cond, "ancestors") {
				t.Fatalf("条件未包含列名: %s", cond)
			}
			if n := strings.Count(cond, "?"); n != len(args) {
				t.Fatalf("占位符数 %d 与参数数 %d 不一致", n, len(args))
			}
			if got := matchCSV(c.csv, cond, args); got != c.want {
				t.Errorf("csv=%q v=%d: got %v, 期望 %v", c.csv, c.v, got, c.want)
			}
		})
	}
}

// TestCSVContainsInt_雪花ID 雪花 ID 是 19 位，必须按十进制原样拼。
func TestCSVContainsInt_雪花ID(t *testing.T) {
	const id int64 = 1761100000000000001
	cond, args := CSVContainsInt("send_user_ids", id)
	for _, a := range args {
		if strings.ContainsAny(a.(string), "eE+") {
			t.Fatalf("参数走了科学计数法: %q (cond=%s)", a, cond)
		}
	}
	csv := "1761100000000000001,1761100000000000002"
	if !matchCSV(csv, cond, args) {
		t.Errorf("雪花 ID 应命中: %q", csv)
	}
	// 相邻 ID 不得互相命中。
	if matchCSV("1761100000000000002", cond, args) {
		t.Error("不同雪花 ID 不应命中")
	}
}
