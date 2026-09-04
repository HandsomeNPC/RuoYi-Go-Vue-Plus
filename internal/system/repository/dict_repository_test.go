package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureDictDataSQL 执行分页查询并捕获 SQL 与绑定变量。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureDictDataSQL(t *testing.T, q bo.SysDictDataQueryBo,
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

	if _, err := NewDictDataRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestDictDataQueryConditions 三个条件全传时都应落到 WHERE：
// 排序号/类型走 =，标签走 LIKE。
func TestDictDataQueryConditions(t *testing.T) {
	sql, vars := captureDictDataSQL(t, bo.SysDictDataQueryBo{
		DictSort:  3,
		DictLabel: "男",
		DictType:  "sys_user_gender",
	}, pkgrepo.PageQuery{})

	for _, want := range []string{
		"dict_sort = ?",
		"dict_label LIKE ?",
		"dict_type = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
	if got, want := len(vars), 3; got != want {
		t.Errorf("绑定变量数 = %d, 期望 %d: %v", got, want, vars)
	}
}

// TestDictDataQueryEmptyConditions 条件全空时不该凭空多出 WHERE 列。
// dict_sort 取 0 视为不筛——排序号从 1 起算，不存在"筛排序号为 0"的诉求。
func TestDictDataQueryEmptyConditions(t *testing.T) {
	sql, vars := captureDictDataSQL(t, bo.SysDictDataQueryBo{}, pkgrepo.PageQuery{})

	for _, unwanted := range []string{"dict_sort = ?", "dict_label LIKE", "dict_type = ?"} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL 不应含 %q:\n%s", unwanted, sql)
		}
	}
	if len(vars) != 0 {
		t.Errorf("不应有绑定变量: %v", vars)
	}
}

// TestDictDataQueryEscapesLike 标签里的 LIKE 元字符须按字面量匹配，
// 不转义的话搜 "%" 会命中全表。
func TestDictDataQueryEscapesLike(t *testing.T) {
	_, vars := captureDictDataSQL(t, bo.SysDictDataQueryBo{DictLabel: "50%_off"},
		pkgrepo.PageQuery{})

	found := false
	for _, v := range vars {
		if s, ok := v.(string); ok && strings.Contains(s, `50\%\_off`) {
			found = true
		}
	}
	if !found {
		t.Errorf("LIKE 元字符未转义: %v", vars)
	}
}

// TestDictDataDefaultOrder 导出路径固定按 dict_sort, dict_code 排序。
//
// 用 SelectList 而非 SelectPageList 验排序：DryRun 下 Count 返回 0 会让 SelectPage
// 提前返回，只捕获得到不带 ORDER BY 的 COUNT 语句。
func TestDictDataDefaultOrder(t *testing.T) {
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
	if _, err := NewDictDataRepository(db).SelectList(t.Context(),
		bo.SysDictDataQueryBo{}, 0); err != nil {
		t.Fatalf("SelectList: %v", err)
	}

	if !strings.Contains(sql, "ORDER BY dict_sort,dict_code") {
		t.Errorf("默认排序不符:\n%s", sql)
	}
}

// TestDictDataSelectByTypeOrder 按类型取字典数据须按排序号升序——
// 前端下拉与标签渲染都依赖这个顺序。
func TestDictDataSelectByTypeOrder(t *testing.T) {
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
	if _, err := NewDictDataRepository(db).SelectByType(t.Context(), "sys_user_gender"); err != nil {
		t.Fatalf("SelectByType: %v", err)
	}

	if !strings.Contains(sql, "dict_type = ?") {
		t.Errorf("SQL 缺少类型过滤:\n%s", sql)
	}
	if !strings.Contains(sql, "ORDER BY dict_sort") {
		t.Errorf("SQL 缺少排序:\n%s", sql)
	}
}

// captureDictTypeSQL 执行分页查询并捕获 SQL 与绑定变量。
func captureDictTypeSQL(t *testing.T, q bo.SysDictTypeQueryBo,
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

	if _, err := NewDictTypeRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestDictTypeQueryConditions 名称与类型都走 LIKE。
func TestDictTypeQueryConditions(t *testing.T) {
	sql, vars := captureDictTypeSQL(t, bo.SysDictTypeQueryBo{
		DictName: "用户",
		DictType: "sys_",
	}, pkgrepo.PageQuery{})

	for _, want := range []string{"dict_name LIKE ?", "dict_type LIKE ?"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
	// dict_type 走 LIKE 而非 =：前端按前缀搜 sys_ 要能命中一批。
	if strings.Contains(sql, "dict_type = ?") {
		t.Errorf("dict_type 应走 LIKE:\n%s", sql)
	}
	if got, want := len(vars), 2; got != want {
		t.Errorf("绑定变量数 = %d, 期望 %d: %v", got, want, vars)
	}
}

// TestDictTypeQueryEmptyConditions 条件全空时不该凭空多出 WHERE 列。
func TestDictTypeQueryEmptyConditions(t *testing.T) {
	sql, vars := captureDictTypeSQL(t, bo.SysDictTypeQueryBo{}, pkgrepo.PageQuery{})

	for _, unwanted := range []string{"dict_name LIKE", "dict_type LIKE"} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL 不应含 %q:\n%s", unwanted, sql)
		}
	}
	if len(vars) != 0 {
		t.Errorf("不应有绑定变量: %v", vars)
	}
}

// TestDictTypeDefaultOrder 导出路径固定按主键升序。
// 用 SelectList 而非 SelectPageList：见 TestDictDataDefaultOrder 的说明。
func TestDictTypeDefaultOrder(t *testing.T) {
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
	if _, err := NewDictTypeRepository(db).SelectList(t.Context(),
		bo.SysDictTypeQueryBo{}, 0); err != nil {
		t.Fatalf("SelectList: %v", err)
	}

	if !strings.Contains(sql, "ORDER BY dict_id") {
		t.Errorf("默认排序不符:\n%s", sql)
	}
}

// TestDictTypeQueryEscapesLike 类型名里的 LIKE 元字符须按字面量匹配。
func TestDictTypeQueryEscapesLike(t *testing.T) {
	_, vars := captureDictTypeSQL(t, bo.SysDictTypeQueryBo{DictType: "sys%_x"},
		pkgrepo.PageQuery{})

	found := false
	for _, v := range vars {
		if s, ok := v.(string); ok && strings.Contains(s, `sys\%\_x`) {
			found = true
		}
	}
	if !found {
		t.Errorf("LIKE 元字符未转义: %v", vars)
	}
}

// TestDictUniqueChecksExcludeSelf 唯一性校验须能排除自身，
// 否则修改时把值改回原样会被误判成冲突。
func TestDictUniqueChecksExcludeSelf(t *testing.T) {
	tests := []struct {
		name      string
		run       func(*gorm.DB) error
		wantNe    string
		wantWhere []string
	}{
		{
			name: "字典数据按类型+键值判重",
			run: func(db *gorm.DB) error {
				_, err := NewDictDataRepository(db).ExistsByTypeAndValue(
					t.Context(), "sys_user_gender", "0", 1761600000000000001)
				return err
			},
			wantNe:    "dict_code <> ?",
			wantWhere: []string{"dict_type = ?", "dict_value = ?"},
		},
		{
			name: "字典类型按类型名判重",
			run: func(db *gorm.DB) error {
				_, err := NewDictTypeRepository(db).ExistsByDictType(
					t.Context(), "sys_user_gender", 1761500000000000001)
				return err
			},
			wantNe:    "dict_id <> ?",
			wantWhere: []string{"dict_type = ?"},
		},
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
			if err := tt.run(db); err != nil {
				t.Fatalf("执行失败: %v", err)
			}

			if !strings.Contains(sql, tt.wantNe) {
				t.Errorf("SQL 缺少排除自身条件 %q:\n%s", tt.wantNe, sql)
			}
			for _, w := range tt.wantWhere {
				if !strings.Contains(sql, w) {
					t.Errorf("SQL 缺少 %q:\n%s", w, sql)
				}
			}
		})
	}
}
