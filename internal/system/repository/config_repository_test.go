package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureConfigSQL 执行分页查询并捕获 SQL 与绑定变量。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureConfigSQL(t *testing.T, q bo.SysConfigQueryBo,
	page pkgrepo.PageQuery) (string, []any) {

	t.Helper()
	db := dryClientDB(t)
	var sql string
	var vars []any
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			if sql == "" {
				sql = tx.Statement.SQL.String()
				vars = tx.Statement.Vars
			}
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewConfigRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestConfigQueryConditions 名称/键名走 like，类型走 eq，时间走闭区间。
func TestConfigQueryConditions(t *testing.T) {
	q := bo.SysConfigQueryBo{
		ConfigName: "初始密码",
		ConfigKey:  "sys.user",
		ConfigType: "Y",
		BeginTime:  "2024-01-01 00:00:00",
		EndTime:    "2024-02-01 23:59:59",
	}
	sql, vars := captureConfigSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, want := range []string{
		"count(*)",
		"FROM `sys_config`",
		"config_name LIKE ?",
		"config_key LIKE ?",
		"config_type = ?",
		"create_time BETWEEN ? AND ?",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL = %s\n应包含 %s", sql, want)
		}
	}

	// like 的通配符须在绑定变量里，不能拼进 SQL 文本。
	if got := vars[0]; got != "%初始密码%" {
		t.Errorf("configName 绑定值 = %v, 期望 %%初始密码%%", got)
	}
	if got := vars[1]; got != "%sys.user%" {
		t.Errorf("configKey 绑定值 = %v, 期望 %%sys.user%%", got)
	}
}

// TestConfigQuerySkipsEmptyConditions 空串条件不参与筛选（likeIfText/eqIfText 语义）。
func TestConfigQuerySkipsEmptyConditions(t *testing.T) {
	sql, _ := captureConfigSQL(t, bo.SysConfigQueryBo{}, pkgrepo.PageQuery{})

	for _, unwanted := range []string{
		"config_name LIKE", "config_key LIKE", "config_type = ?", "create_time BETWEEN",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL = %s\n不应包含 %s", sql, unwanted)
		}
	}
	if !strings.Contains(sql, "FROM `sys_config`") {
		t.Errorf("SQL = %s\n应包含 FROM `sys_config`", sql)
	}
}

// TestConfigQueryTimeRangeNeedsBothEnds 只给一端时间不筛，
// 对齐 Java betweenParams 的 begin != null && end != null。
func TestConfigQueryTimeRangeNeedsBothEnds(t *testing.T) {
	for name, q := range map[string]bo.SysConfigQueryBo{
		"只有起始": {BeginTime: "2024-01-01 00:00:00"},
		"只有结束": {EndTime: "2024-02-01 23:59:59"},
	} {
		t.Run(name, func(t *testing.T) {
			sql, _ := captureConfigSQL(t, q, pkgrepo.PageQuery{})
			if strings.Contains(sql, "create_time") {
				t.Errorf("SQL = %s\n不应包含 create_time 条件", sql)
			}
		})
	}
}

// TestConfigListDefaultOrder 导出路径固定按主键升序，保证输出顺序稳定。
func TestConfigListDefaultOrder(t *testing.T) {
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewConfigRepository(db).SelectList(t.Context(), bo.SysConfigQueryBo{}, 100); err != nil {
		t.Fatalf("SelectList: %v", err)
	}
	// LIMIT 值是绑定变量而非字面量，故只断言子句存在。
	for _, want := range []string{"ORDER BY config_id", "LIMIT ?"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL = %s\n应包含 %s", sql, want)
		}
	}
}

// TestConfigQueryDefaultOrder 调用方已指定排序时不得再追加 config_id，
// 否则主键唯一会让调用方的排序列失效。
func TestConfigQueryDefaultOrder(t *testing.T) {
	sql, _ := captureConfigSQL(t, bo.SysConfigQueryBo{},
		pkgrepo.PageQuery{OrderByColumn: "configName", IsAsc: "desc"})
	if strings.Contains(sql, "config_id") {
		t.Errorf("SQL = %s\n调用方已指定排序，不应再兜底 config_id", sql)
	}
}

// TestEscapeLike LIKE 元字符须转义，否则搜 "%" 会命中全表。
func TestEscapeLike(t *testing.T) {
	tests := []struct{ in, want string }{
		{"初始密码", "初始密码"},
		{"sys.user", "sys.user"},
		{"100%", `100\%`},
		{"a_b", `a\_b`},
		// 反斜杠先转，故输入的单个 \ 变成 \\ 而非被后续步骤重复转义。
		{`a\b`, `a\\b`},
		{`%_\`, `\%\_\\`},
	}
	for _, tt := range tests {
		if got := escapeLike(tt.in); got != tt.want {
			t.Errorf("escapeLike(%q) = %q, 期望 %q", tt.in, got, tt.want)
		}
	}
}
