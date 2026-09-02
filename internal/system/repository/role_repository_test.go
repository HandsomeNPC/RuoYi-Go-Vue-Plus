package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureRoleSQL 在 DryRun 模式下执行角色分页查询并捕获途中所有 SQL。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureRoleSQL(t *testing.T, q bo.SysRoleQueryBo, page pkgrepo.PageQuery) []string {
	t.Helper()
	db := dryClientDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:captureRole", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewRoleRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	return sqls
}

// TestSelectRolePageListWhereConditions 四个条件全传时都应落到 WHERE，且保留逻辑删除过滤。
func TestSelectRolePageListWhereConditions(t *testing.T) {
	q := bo.SysRoleQueryBo{
		RoleID:    1761300000000000001,
		RoleName:  "admin",
		RoleKey:   "system",
		Status:    "0",
		BeginTime: "2026-01-01 00:00:00",
		EndTime:   "2026-01-31 23:59:59",
	}
	got := captureRoleSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]

	for _, want := range []string{
		"count(*)",
		"FROM `sys_role`",
		"role_id = ?",
		"role_name LIKE ?",
		"role_key LIKE ?",
		"status = ?",
		"create_time BETWEEN ? AND ?",
		"del_flag",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
}

// TestSelectRolePageListEmptySkipsFilter 空串/零值条件不应落到 WHERE——对齐 Java likeIfText/eqIfText。
func TestSelectRolePageListEmptySkipsFilter(t *testing.T) {
	got := captureRoleSQL(t, bo.SysRoleQueryBo{},
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]

	for _, unwanted := range []string{
		"role_id = ?", "role_name LIKE", "role_key LIKE", "status = ?", "create_time BETWEEN",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("SQL = %s\n不应包含 %s", got, unwanted)
		}
	}
	// 逻辑删除条件与表名仍须在。
	if !strings.Contains(got, "del_flag") || !strings.Contains(got, "FROM `sys_role`") {
		t.Errorf("SQL = %s\n应保留 del_flag 与 sys_role", got)
	}
}

// TestSelectRolePageListHalfTimeRangeNotFilter 只给一端时间不应筛——
// 前端清空半个日期框时结果不该突变，对齐 Java betweenParams 的 begin && end 语义。
func TestSelectRolePageListHalfTimeRangeNotFilter(t *testing.T) {
	got := captureRoleSQL(t, bo.SysRoleQueryBo{BeginTime: "2026-01-01 00:00:00"},
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]
	if strings.Contains(got, "create_time BETWEEN") {
		t.Errorf("SQL = %s\n只给一端时间不应筛 create_time", got)
	}
}

// TestEscapeRoleLike LIKE 元字符须转义成字面量——
// 不转义搜 % 会命中全表、_ 会变成任意单字符通配，这是与 Java likeIfText 的有意差异。
// 反斜杠排最前，避免重复转义。
func TestEscapeRoleLike(t *testing.T) {
	cases := map[string]string{
		"%_admin": `\%\_admin`,
		`a\b%c_d`: `a\\b\%c\_d`,
		"normal":  "normal",
		"":        "",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
