package repository

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// dryClientDB DryRun 模式的 gorm 实例，只拼 SQL 不连库。
// pkg/repository 里的同名辅助是包内测试代码，跨包取不到，故本包自备。
func dryClientDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "dry:run@tcp(127.0.0.1:1)/dry?charset=utf8mb4",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		NamingStrategy:         schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatalf("构造 DryRun 实例失败: %v", err)
	}
	return db
}

// captureSQL 执行分页查询并捕获途中所有 SQL。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureSQL(t *testing.T, q bo.SysClientQueryBo, page pkgrepo.PageQuery) []string {
	t.Helper()
	db := dryClientDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewClientRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	return sqls
}

// TestSelectPageListWhereConditions 四个条件全传时都应落到 WHERE，且保留逻辑删除过滤。
func TestSelectPageListWhereConditions(t *testing.T) {
	q := bo.SysClientQueryBo{
		ClientID:     "e5cd7e4891bf95d1d19206ce24a7b32e",
		ClientKey:    "pc",
		ClientSecret: "pc123",
		Status:       "0",
	}
	got := captureSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]

	for _, want := range []string{
		"count(*)",
		"FROM `sys_client`",
		"client_id = ?",
		"client_key = ?",
		"client_secret = ?",
		"status = ?",
		"del_flag",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
}

// TestSelectPageListSkipsEmptyConditions 空串条件不参与筛选（eqIfText 语义）。
func TestSelectPageListSkipsEmptyConditions(t *testing.T) {
	got := captureSQL(t, bo.SysClientQueryBo{}, pkgrepo.PageQuery{})[0]

	for _, unwanted := range []string{"client_id = ?", "client_key = ?", "client_secret = ?", "status = ?"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("SQL = %s\n不应包含 %s", got, unwanted)
		}
	}
	// 逻辑删除条件与表名仍须在。
	if !strings.Contains(got, "del_flag") || !strings.Contains(got, "FROM `sys_client`") {
		t.Errorf("SQL = %s\n应保留 del_flag 与 sys_client", got)
	}
}

// TestSelectPageListDefaultOrderByID 未指定排序时兜底按主键升序。
//
// 不断言 SQL：Count 执行前会摘掉 ORDER BY（gorm finisher_api），且 DryRun 下 total 恒 0
// 使 SelectPage 在 Find 之前返回，排序子句在这条路径上无从观测。故退一步断言驱动该分支的
// 判定本身——补 id 与否完全由 HasOrder 决定，两个用例合起来锁住这个契约。
func TestSelectPageListDefaultOrderByID(t *testing.T) {
	for _, page := range []pkgrepo.PageQuery{
		{PageNum: 1, PageSize: 10},                                 // 全空
		{PageNum: 1, PageSize: 10, OrderByColumn: "clientKey"},     // 只给列
		{PageNum: 1, PageSize: 10, IsAsc: "desc"},                  // 只给方向
		{PageNum: 1, PageSize: 10, OrderByColumn: " ", IsAsc: " "}, // 纯空白
	} {
		if page.HasOrder() {
			t.Errorf("PageQuery%+v 应视为未指定排序（须补 id 兜底）", page)
		}
		// 顺带确认这些入参不会让 SelectPageList 报错。
		if _, err := NewClientRepository(dryClientDB(t)).
			SelectPageList(t.Context(), bo.SysClientQueryBo{}, page); err != nil {
			t.Errorf("PageQuery%+v: %v", page, err)
		}
	}
}

// TestSelectPageListRespectsPageQueryOrder 调用方指定排序时不得再追加 id。
// GORM 合并排序子句时先注册的排在前，若无条件补 id，唯一主键会让指定的排序列彻底失效。
func TestSelectPageListRespectsPageQueryOrder(t *testing.T) {
	page := pkgrepo.PageQuery{PageNum: 1, PageSize: 10, OrderByColumn: "clientKey", IsAsc: "desc"}
	if !page.HasOrder() {
		t.Fatal("列与方向都给了，应视为已指定排序（不补 id）")
	}

	// 已指定排序时须落成 client_key DESC，且不掺入 id。
	order, err := page.OrderBy()
	if err != nil {
		t.Fatalf("OrderBy: %v", err)
	}
	if len(order.Columns) != 1 {
		t.Fatalf("排序列数 = %d, 期望 1: %+v", len(order.Columns), order.Columns)
	}
	if got := order.Columns[0].Column.Name; got != "client_key" {
		t.Errorf("排序列 = %q, 期望 client_key（驼峰应转下划线）", got)
	}
	if !order.Columns[0].Desc {
		t.Error("isAsc=desc 应降序")
	}
}

// TestSelectPageListRejectsInjectedOrder 非法排序列须在打库前被拒。
func TestSelectPageListRejectsInjectedOrder(t *testing.T) {
	db := dryClientDB(t)
	queried := false
	if err := db.Callback().Query().After("gorm:query").
		Register("test:flag", func(*gorm.DB) { queried = true }); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	page := pkgrepo.PageQuery{PageNum: 1, PageSize: 10, OrderByColumn: "id;DROP TABLE x", IsAsc: "asc"}
	res, err := NewClientRepository(db).SelectPageList(t.Context(), bo.SysClientQueryBo{}, page)
	if err == nil {
		t.Fatal("非法排序参数应返回错误")
	}
	if queried {
		t.Error("排序参数非法时不应执行查询")
	}
	if res.Total != 0 || res.Rows == nil {
		t.Errorf("失败时应返回空页, got %+v", res)
	}
}

// captureCountSQL 执行唯一性校验并捕获其 COUNT 语句。
func captureCountSQL(t *testing.T, clientKey string, excludeID int64) string {
	t.Helper()
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_count", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewClientRepository(db).ExistsByClientKey(t.Context(), clientKey, excludeID); err != nil {
		t.Fatalf("ExistsByClientKey: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql
}

// TestExistsByClientKeySQL 唯一性校验须带 del_flag 过滤——已逻辑删除的行不该占用 client_key。
func TestExistsByClientKeySQL(t *testing.T) {
	got := captureCountSQL(t, "pc", 0)

	for _, want := range []string{"count(*)", "FROM `sys_client`", "client_key = ?", "del_flag"} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
	// 新增场景无自身可排除，不应出现 id <> ?。
	if strings.Contains(got, "id <> ?") {
		t.Errorf("SQL = %s\nexcludeID=0 时不应排除主键", got)
	}
}

// TestExistsByClientKeyExcludeID excludeID > 0 时排除自身（供修改场景复用）。
func TestExistsByClientKeyExcludeID(t *testing.T) {
	got := captureCountSQL(t, "pc", 1762000000000000001)

	if !strings.Contains(got, "id <> ?") {
		t.Errorf("SQL = %s\n应包含 id <> ? 以排除自身", got)
	}
}

// TestExistsByClientKeyEmptyKey 空 key 直接判为未占用，不打库。
func TestExistsByClientKeyEmptyKey(t *testing.T) {
	db := dryClientDB(t)
	queried := false
	if err := db.Callback().Query().After("gorm:query").
		Register("test:flag", func(*gorm.DB) { queried = true }); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	exists, err := NewClientRepository(db).ExistsByClientKey(t.Context(), "", 0)
	if err != nil {
		t.Fatalf("ExistsByClientKey: %v", err)
	}
	if exists {
		t.Error("空 key 应判为未占用")
	}
	if queried {
		t.Error("空 key 不应打库")
	}
}

// TestInsertNilClient nil 入参须返回错误而非 panic。
func TestInsertNilClient(t *testing.T) {
	if err := NewClientRepository(dryClientDB(t)).Insert(t.Context(), nil); err == nil {
		t.Error("nil 客户端应返回错误")
	}
}
