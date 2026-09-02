package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// capturePostSQL 执行分页查询并捕获 SQL 与绑定变量。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func capturePostSQL(t *testing.T, q bo.SysPostQueryBo,
	page pkgrepo.PageQuery) (string, []any) {

	t.Helper()
	db := dryClientDB(t)
	var sql string
	var vars []any
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture-post", func(tx *gorm.DB) {
			if sql == "" {
				sql = tx.Statement.SQL.String()
				vars = tx.Statement.Vars
			}
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewPostRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestPostQueryConditions 编码/类别/名称走 LIKE，状态走 =，时间走闭区间。
func TestPostQueryConditions(t *testing.T) {
	q := bo.SysPostQueryBo{
		PostCode:     "ceo",
		PostCategory: "tech",
		PostName:     "董事长",
		Status:       "0",
		BeginTime:    "2024-01-01 00:00:00",
		EndTime:      "2024-02-01 23:59:59",
	}
	sql, vars := capturePostSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, want := range []string{
		"count(*)",
		"FROM `sys_post`",
		"post_code LIKE ?",
		"post_category LIKE ?",
		"post_name LIKE ?",
		"status = ?",
		"create_time BETWEEN ? AND ?",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL = %s\n应包含 %s", sql, want)
		}
	}

	// like 的通配符须在绑定变量里，不能拼进 SQL 文本。
	if got := vars[0]; got != "%ceo%" {
		t.Errorf("postCode 绑定值 = %v, 期望 %%ceo%%", got)
	}
	if got := vars[1]; got != "%tech%" {
		t.Errorf("postCategory 绑定值 = %v, 期望 %%tech%%", got)
	}
	if got := vars[2]; got != "%董事长%" {
		t.Errorf("postName 绑定值 = %v, 期望 %%董事长%%", got)
	}
}

// TestPostQuerySkipsEmptyConditions 空串条件不参与筛选（likeIfText/eqIfText 语义）。
func TestPostQuerySkipsEmptyConditions(t *testing.T) {
	sql, _ := capturePostSQL(t, bo.SysPostQueryBo{}, pkgrepo.PageQuery{})

	for _, unwanted := range []string{
		"post_code LIKE", "post_category LIKE", "post_name LIKE",
		"status = ?", "create_time BETWEEN", "dept_id",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL = %s\n不应包含 %s", sql, unwanted)
		}
	}
	if !strings.Contains(sql, "FROM `sys_post`") {
		t.Errorf("SQL = %s\n应包含 FROM `sys_post`", sql)
	}
}

// TestPostQueryTimeRangeNeedsBothEnds 只给一端时间不筛，
// 对齐 Java betweenParams 的 begin != null && end != null。
func TestPostQueryTimeRangeNeedsBothEnds(t *testing.T) {
	for name, q := range map[string]bo.SysPostQueryBo{
		"只有起始": {BeginTime: "2024-01-01 00:00:00"},
		"只有结束": {EndTime: "2024-02-01 23:59:59"},
	} {
		t.Run(name, func(t *testing.T) {
			sql, _ := capturePostSQL(t, q, pkgrepo.PageQuery{})
			if strings.Contains(sql, "create_time") {
				t.Errorf("SQL = %s\n不应包含 create_time 条件", sql)
			}
		})
	}
}

// TestPostQueryDeptTreePrecedence 单部门搜索优先于部门树搜索（对齐 Java 的 if/else if）。
// 两者同时给出时只走 dept_id = ?，不应出现 dept_id IN。
func TestPostQueryDeptTreePrecedence(t *testing.T) {
	q := bo.SysPostQueryBo{DeptID: 100, DeptIDs: []int64{100, 101}}
	sql, _ := capturePostSQL(t, q, pkgrepo.PageQuery{})

	if !strings.Contains(sql, "dept_id = ?") {
		t.Errorf("SQL = %s\n应包含 dept_id = ?", sql)
	}
	if strings.Contains(sql, "dept_id IN ?") {
		t.Errorf("SQL = %s\n单部门优先，不应出现 dept_id IN", sql)
	}
}

// TestPostQueryDeptTreeIN 部门树搜索走 IN（DeptIDs 由 service 按 BelongDeptID 解析后回填）。
// GORM 把切片展开成 IN (?,?,?)，故只断言 IN 子句出现而非占位符形态。
func TestPostQueryDeptTreeIN(t *testing.T) {
	q := bo.SysPostQueryBo{DeptIDs: []int64{100, 101, 102}}
	sql, _ := capturePostSQL(t, q, pkgrepo.PageQuery{})
	if !strings.Contains(sql, "dept_id IN") {
		t.Errorf("SQL = %s\n应包含 dept_id IN", sql)
	}
}

// TestPostQueryDefaultOrderSkipWhenSpecified 调用方已指定排序时不得再追加 post_sort。
func TestPostQueryDefaultOrderSkipWhenSpecified(t *testing.T) {
	sql, _ := capturePostSQL(t, bo.SysPostQueryBo{},
		pkgrepo.PageQuery{OrderByColumn: "postName", IsAsc: "desc"})
	if strings.Contains(sql, "post_sort") {
		t.Errorf("SQL = %s\n调用方已指定排序，不应再兜底 post_sort", sql)
	}
}

// TestPostListDefaultOrder 导出路径固定按 post_sort 升序，保证输出顺序稳定。
func TestPostListDefaultOrder(t *testing.T) {
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture-post-list", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewPostRepository(db).SelectList(t.Context(), bo.SysPostQueryBo{}, 100); err != nil {
		t.Fatalf("SelectList: %v", err)
	}
	for _, want := range []string{"ORDER BY post_sort", "LIMIT ?"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL = %s\n应包含 %s", sql, want)
		}
	}
}
