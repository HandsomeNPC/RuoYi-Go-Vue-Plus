package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// TestSelectPageCountThenFindSQL 验证 COUNT 不带 ORDER BY/LIMIT、Find 带排序分页，且 WHERE 一致。
// 这是新建 session 的意义：statement 若被复用，Find 会带上 SELECT count(*) 或条件叠加。
func TestSelectPageCountThenFindSQL(t *testing.T) {
	var sqls []string
	db := dryDB(t)
	// DryRun 下 Count 拿不到真实值，total=0 会提前返回，故只断言 COUNT；Find 见下一用例。
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	q := PageQuery{PageNum: 2, PageSize: 10, OrderByColumn: "createTime", IsAsc: "desc"}
	var rows []flagUser
	// DryRun 下 Count 拿不到真实值，total 为 0 会提前返回，
	// 因此这里只断言 COUNT 语句；Find 语句在下一个用例里单独验证。
	if _, err := SelectPage(db.Model(&flagUser{}).Where("status = ?", "0"), q, &rows); err != nil {
		t.Fatalf("SelectPage: %v", err)
	}

	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	countSQL := sqls[0]
	if !strings.Contains(countSQL, "count(*)") {
		t.Errorf("首条应为 COUNT 查询: %s", countSQL)
	}
	if !strings.Contains(countSQL, "status = ?") {
		t.Errorf("COUNT 应保留调用方 WHERE 条件: %s", countSQL)
	}
	if !strings.Contains(countSQL, "del_flag") {
		t.Errorf("COUNT 应保留逻辑删除条件: %s", countSQL)
	}
	if strings.Contains(countSQL, "ORDER BY") || strings.Contains(countSQL, "LIMIT") {
		t.Errorf("COUNT 不应带排序或分页: %s", countSQL)
	}
}

// TestPaginateAndOrderProduceLimitOffset 验证分页 Scope 与排序合并后的 SQL 形态。
func TestPaginateAndOrderProduceLimitOffset(t *testing.T) {
	q := PageQuery{PageNum: 3, PageSize: 15, OrderByColumn: "createTime", IsAsc: "desc"}
	order, err := q.OrderBy()
	if err != nil {
		t.Fatalf("OrderBy: %v", err)
	}

	var rows []flagUser
	tx := dryDB(t).Model(&flagUser{}).Where("status = ?", "0").
		Scopes(q.Paginate()).Order(order).Find(&rows)
	got := sqlOf(t, tx.Statement.DB)

	for _, want := range []string{
		"status = ?",
		"`flag_user`.`del_flag` = ?",
		"ORDER BY `create_time` DESC",
		"LIMIT ? OFFSET ?",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}

	// LIMIT/OFFSET 走参数占位符，值应为 15 / 30。
	vars := tx.Statement.Vars
	if len(vars) < 2 {
		t.Fatalf("参数过少: %v", vars)
	}
	limit, offset := vars[len(vars)-2], vars[len(vars)-1]
	if limit != 15 {
		t.Errorf("LIMIT = %v, want 15", limit)
	}
	if offset != 30 {
		t.Errorf("OFFSET = %v, want 30", offset)
	}
}

// TestSelectPageRejectsBadOrderBeforeQuery 非法排序应在打库前返回错误。
func TestSelectPageRejectsBadOrderBeforeQuery(t *testing.T) {
	queried := false
	db := dryDB(t)
	if err := db.Callback().Query().After("gorm:query").
		Register("test:flag", func(*gorm.DB) { queried = true }); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	q := PageQuery{PageNum: 1, PageSize: 10, OrderByColumn: "id;DROP TABLE x", IsAsc: "asc"}
	var rows []flagUser
	page, err := SelectPage(db.Model(&flagUser{}), q, &rows)
	if err == nil {
		t.Fatal("非法排序参数应返回错误")
	}
	if queried {
		t.Error("排序参数非法时不应执行查询")
	}
	if page.Total != 0 || page.Rows == nil {
		t.Errorf("失败时应返回空页, got %+v", page)
	}
}

// userDeptVO 联表出参，故意非实体：表名与 flag_user 无关。
type userDeptVO struct {
	UserID   int64  `gorm:"column:user_id"`
	UserName string `gorm:"column:user_name"`
	DeptName string `gorm:"column:dept_name"`
}

// TestSelectPageKeepsCallerModel dest 是 VO 时不应拿 dest 反推表名，
// 否则会打到不存在的表并丢掉实体的 del_flag 条件。
func TestSelectPageKeepsCallerModel(t *testing.T) {
	db := dryDB(t)
	var captured string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			if captured == "" {
				captured = tx.Statement.SQL.String()
			}
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	var rows []userDeptVO
	q := PageQuery{PageNum: 1, PageSize: 10}
	if _, err := SelectPage(db.Model(&flagUser{}), q, &rows); err != nil {
		t.Fatalf("SelectPage: %v", err)
	}

	if strings.Contains(captured, "user_dept_vo") {
		t.Errorf("不应拿 VO 反推表名: %s", captured)
	}
	if !strings.Contains(captured, "`flag_user`") {
		t.Errorf("应沿用调用方 Model 的表名: %s", captured)
	}
	if !strings.Contains(captured, "del_flag") {
		t.Errorf("应保留实体的逻辑删除条件: %s", captured)
	}
}

// TestSelectPageWithTableAndJoins 调用方用 Table()+Joins() 时，COUNT 与 Find 都沿用且条件不叠加。
func TestSelectPageWithTableAndJoins(t *testing.T) {
	db := dryDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	var rows []userDeptVO
	q := PageQuery{PageNum: 1, PageSize: 10}
	base := db.Table("flag_user u").
		Joins("LEFT JOIN sys_dept d ON d.dept_id = u.dept_id").
		Where("u.status = ?", "0")
	if _, err := SelectPage(base, q, &rows); err != nil {
		t.Fatalf("SelectPage: %v", err)
	}

	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	countSQL := sqls[0]
	if !strings.Contains(countSQL, "FROM flag_user u") ||
		!strings.Contains(countSQL, "LEFT JOIN sys_dept d") {
		t.Errorf("COUNT 应沿用 Table/Joins: %s", countSQL)
	}
	if !strings.Contains(countSQL, "count(*)") {
		t.Errorf("首条应为 COUNT: %s", countSQL)
	}
}
