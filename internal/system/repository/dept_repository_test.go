package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
)

// captureDeptSQL 执行列表查询并捕获 SQL 与绑定变量。
func captureDeptSQL(t *testing.T, q bo.SysDeptQueryBo) (string, []any) {
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

	if _, err := NewDeptRepository(db).SelectList(t.Context(), q); err != nil {
		t.Fatalf("SelectList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestDeptSelectListWhereConditions 条件全传时都应落到 WHERE，且保留逻辑删除过滤。
func TestDeptSelectListWhereConditions(t *testing.T) {
	sql, vars := captureDeptSQL(t, bo.SysDeptQueryBo{
		DeptID:       1761000000000000103,
		ParentID:     1761000000000000101,
		DeptName:     "研发",
		DeptCategory: "RD",
		Status:       "0",
		BelongDeptID: 1761000000000000100,
		BeginTime:    "2026-01-01 00:00:00",
		EndTime:      "2026-01-31 23:59:59",
	})

	for _, want := range []string{
		"dept_id = ?",
		"parent_id = ?",
		"dept_name LIKE ?",
		"dept_category LIKE ?",
		"status = ?",
		"create_time BETWEEN ? AND ?",
		"FIND_IN_SET(?, ancestors)",
		"`del_flag` = ?", // LogicDelete 自动追加
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
	// 祖先链在前保证父节点先于子节点出现，前端构树依赖这个顺序。
	if !strings.Contains(sql, "ORDER BY ancestors,parent_id,order_num,dept_id") {
		t.Errorf("排序不符:\n%s", sql)
	}
	if got, want := len(vars), 10; got != want {
		t.Errorf("绑定变量数 = %d, 期望 %d: %v", got, want, vars)
	}
}

// TestDeptSelectListEmptyConditions 条件全空时只剩逻辑删除过滤，不该凭空多出 WHERE 列。
func TestDeptSelectListEmptyConditions(t *testing.T) {
	sql, _ := captureDeptSQL(t, bo.SysDeptQueryBo{})

	for _, unwanted := range []string{
		"dept_id = ?",
		"parent_id = ?",
		"dept_name LIKE",
		"dept_category LIKE",
		"status = ?",
		"create_time BETWEEN",
		"FIND_IN_SET",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL 不应含 %q:\n%s", unwanted, sql)
		}
	}
	if !strings.Contains(sql, "`del_flag` = ?") {
		t.Errorf("SQL 应保留逻辑删除过滤:\n%s", sql)
	}
}

// TestDeptSelectListTimeRangeNeedsBothEnds 时间区间只给一端时不筛——
// 否则前端清空半个日期框会让结果突变。
func TestDeptSelectListTimeRangeNeedsBothEnds(t *testing.T) {
	for name, q := range map[string]bo.SysDeptQueryBo{
		"只给开始": {BeginTime: "2026-01-01 00:00:00"},
		"只给结束": {EndTime: "2026-01-31 23:59:59"},
	} {
		t.Run(name, func(t *testing.T) {
			sql, _ := captureDeptSQL(t, q)
			if strings.Contains(sql, "create_time BETWEEN") {
				t.Errorf("单端时间区间不应生效:\n%s", sql)
			}
		})
	}
}

// TestDeptSelectListEscapesLike 部门名里的 LIKE 元字符须按字面量匹配，
// 不转义的话搜 "%" 会命中全表。
func TestDeptSelectListEscapesLike(t *testing.T) {
	_, vars := captureDeptSQL(t, bo.SysDeptQueryBo{DeptName: "100%_a"})

	found := false
	for _, v := range vars {
		if s, ok := v.(string); ok && strings.Contains(s, `100\%\_a`) {
			found = true
		}
	}
	if !found {
		t.Errorf("LIKE 元字符未转义: %v", vars)
	}
}

// TestDeptSelectListBelongDeptIncludesSelf 部门树搜索须连自身一起命中，
// 只查后代会让搜索结果缺掉被搜的那个部门。
func TestDeptSelectListBelongDeptIncludesSelf(t *testing.T) {
	sql, _ := captureDeptSQL(t, bo.SysDeptQueryBo{BelongDeptID: 1761000000000000100})

	if !strings.Contains(sql, "dept_id = ? OR FIND_IN_SET(?, ancestors)") {
		t.Errorf("部门树搜索应含自身:\n%s", sql)
	}
}

// TestDeptSelectNormalByIDsOptionalFilter ids 为空时退化成查全部启用部门，
// 非空时才加 IN。
func TestDeptSelectNormalByIDsOptionalFilter(t *testing.T) {
	tests := []struct {
		name   string
		ids    []int64
		wantIn bool
	}{
		{"无 ids", nil, false},
		{"有 ids", []int64{1761000000000000100, 1761000000000000101}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := dryClientDB(t)
			var sql string
			if err := db.Callback().Query().After("gorm:query").
				Register("test:capture", func(tx *gorm.DB) {
					if sql == "" {
						sql = tx.Statement.SQL.String()
					}
				}); err != nil {
				t.Fatalf("注册 callback 失败: %v", err)
			}
			if _, err := NewDeptRepository(db).SelectNormalByIDs(t.Context(), tt.ids); err != nil {
				t.Fatalf("SelectNormalByIDs: %v", err)
			}

			if got := strings.Contains(sql, "dept_id IN"); got != tt.wantIn {
				t.Errorf("含 IN 条件 = %v, 期望 %v:\n%s", got, tt.wantIn, sql)
			}
			if !strings.Contains(sql, "status = ?") {
				t.Errorf("应只查启用部门:\n%s", sql)
			}
		})
	}
}
