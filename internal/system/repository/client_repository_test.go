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

// captureUpdateSQL 执行更新并捕获其 SQL。
func captureUpdateSQL(t *testing.T, run func(*ClientRepository) error) string {
	t.Helper()
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Update().After("gorm:update").
		Register("test:capture_update", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if err := run(NewClientRepository(db)); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql
}

// TestUpdateByIDSQL 更新须按主键定位并带 del_flag 过滤——已逻辑删除的行不该被改回来。
func TestUpdateByIDSQL(t *testing.T) {
	got := captureUpdateSQL(t, func(r *ClientRepository) error {
		_, err := r.UpdateByID(t.Context(), 1762000000000000001, map[string]any{
			"client_key":   "pc",
			"access_path":  "",
			"ip_whitelist": "",
		})
		return err
	})

	for _, want := range []string{"UPDATE `sys_client`", "`client_key`", "id = ?", "del_flag"} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
}

// TestUpdateByIDWritesEmptyStrings 空串列必须进 SET 子句。
// 这是走 map 而非 Updates(struct) 的全部理由：后者跳过零值，
// 会让前端「清空访问路径/IP 白名单」的操作静默丢失。
func TestUpdateByIDWritesEmptyStrings(t *testing.T) {
	got := captureUpdateSQL(t, func(r *ClientRepository) error {
		_, err := r.UpdateByID(t.Context(), 1762000000000000001, map[string]any{
			"access_path":  "",
			"ip_whitelist": "",
		})
		return err
	})

	for _, want := range []string{"`access_path`", "`ip_whitelist`"} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n空串列 %s 也须写入(清空语义)", got, want)
		}
	}
}

// TestUpdateByIDFillsAuditColumns 审计列由 pkg/repository 的更新回调补齐，调用方不必带。
func TestUpdateByIDFillsAuditColumns(t *testing.T) {
	db := dryClientDB(t)
	if err := pkgrepo.RegisterAuditCallbacks(db); err != nil {
		t.Fatalf("注册审计回调失败: %v", err)
	}
	var sql string
	if err := db.Callback().Update().After("gorm:update").
		Register("test:capture_audit", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewClientRepository(db).UpdateByID(t.Context(), 1762000000000000001,
		map[string]any{"client_key": "pc"}); err != nil {
		t.Fatalf("UpdateByID: %v", err)
	}

	for _, want := range []string{"`update_by`", "`update_time`"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL = %s\n应由审计回调补上 %s", sql, want)
		}
	}
}

// TestUpdateByIDRejectsBadInput 主键无效或无列可更新时不打库。
func TestUpdateByIDRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		columns map[string]any
	}{
		{"主键为 0", 0, map[string]any{"status": "1"}},
		{"主键为负", -1, map[string]any{"status": "1"}},
		{"列为空 map", 1, map[string]any{}},
		{"列为 nil", 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := dryClientDB(t)
			updated := false
			if err := db.Callback().Update().After("gorm:update").
				Register("test:flag", func(*gorm.DB) { updated = true }); err != nil {
				t.Fatalf("注册 callback 失败: %v", err)
			}

			affected, err := NewClientRepository(db).UpdateByID(t.Context(), tt.id, tt.columns)
			if err == nil {
				t.Error("应返回错误")
			}
			if affected != 0 {
				t.Errorf("affected = %d, 期望 0", affected)
			}
			if updated {
				t.Error("入参非法时不应打库")
			}
		})
	}
}

// TestUpdateStatusByClientIDSQL 改状态须按 client_id 定位，且只动 status。
func TestUpdateStatusByClientIDSQL(t *testing.T) {
	got := captureUpdateSQL(t, func(r *ClientRepository) error {
		_, err := r.UpdateStatusByClientID(t.Context(), "e5cd7e4891bf95d1d19206ce24a7b32e", "1")
		return err
	})

	for _, want := range []string{"UPDATE `sys_client`", "`status`", "client_id = ?", "del_flag"} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
	// 不得连带改动其他业务列。
	for _, unwanted := range []string{"`client_key`", "`client_secret`", "`grant_type`"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("SQL = %s\n改状态不应触及 %s", got, unwanted)
		}
	}
}

// captureDeleteSQL 执行删除并捕获其 SQL。
// 钩子挂在 Delete 链上：逻辑删除虽把语句改写成 UPDATE，走的仍是 Delete processor。
func captureDeleteSQL(t *testing.T, ids []int64) string {
	t.Helper()
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Delete().After("gorm:delete").
		Register("test:capture_delete", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewClientRepository(db).DeleteByIDs(t.Context(), ids); err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql
}

// TestDeleteByIDsIsLogicDelete 删除须落成 UPDATE del_flag='1' 而非物理 DELETE，
// 且带 del_flag='0' 过滤——重复删除同一批主键第二次应报 0 行。
func TestDeleteByIDsIsLogicDelete(t *testing.T) {
	got := captureDeleteSQL(t, []int64{1762000000000000001, 1762000000000000002})

	for _, want := range []string{"UPDATE `sys_client`", "`del_flag`", "id IN (?,?)"} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
	if strings.Contains(got, "DELETE FROM") {
		t.Errorf("SQL = %s\n不应物理删除", got)
	}
	// del_flag 既在 SET 也在 WHERE：SET 置 '1'，WHERE 过滤 '0'。
	if strings.Count(got, "del_flag") < 2 {
		t.Errorf("SQL = %s\ndel_flag 应同时出现在 SET 与 WHERE", got)
	}
}

// TestDeleteByIDsSingleID 单个主键也走 IN，不因长度 1 退化。
func TestDeleteByIDsSingleID(t *testing.T) {
	got := captureDeleteSQL(t, []int64{1762000000000000001})

	if !strings.Contains(got, "id IN (?)") {
		t.Errorf("SQL = %s\n应包含 id IN (?)", got)
	}
}

// TestDeleteByIDsEmpty 空主键须返回错误且不打库——否则 WHERE 落空会软删全表。
func TestDeleteByIDsEmpty(t *testing.T) {
	for _, name := range []string{"nil", "空切片"} {
		t.Run(name, func(t *testing.T) {
			var ids []int64
			if name == "空切片" {
				ids = []int64{}
			}

			db := dryClientDB(t)
			touched := false
			flag := func(*gorm.DB) { touched = true }
			if err := db.Callback().Update().After("gorm:update").
				Register("test:flag_u", flag); err != nil {
				t.Fatalf("注册 callback 失败: %v", err)
			}
			if err := db.Callback().Delete().After("gorm:delete").
				Register("test:flag_d", flag); err != nil {
				t.Fatalf("注册 callback 失败: %v", err)
			}

			affected, err := NewClientRepository(db).DeleteByIDs(t.Context(), ids)
			if err == nil {
				t.Error("空主键应返回错误")
			}
			if affected != 0 {
				t.Errorf("affected = %d, 期望 0", affected)
			}
			if touched {
				t.Error("空主键不应打库")
			}
		})
	}
}

// TestUpdateStatusByClientIDEmptyID 空标识须返回错误且不打库——否则 WHERE 落空会全表刷状态。
func TestUpdateStatusByClientIDEmptyID(t *testing.T) {
	db := dryClientDB(t)
	updated := false
	if err := db.Callback().Update().After("gorm:update").
		Register("test:flag", func(*gorm.DB) { updated = true }); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	affected, err := NewClientRepository(db).UpdateStatusByClientID(t.Context(), "", "1")
	if err == nil {
		t.Error("空客户端标识应返回错误")
	}
	if affected != 0 {
		t.Errorf("affected = %d, 期望 0", affected)
	}
	if updated {
		t.Error("空客户端标识不应打库")
	}
}

// captureListSQL 执行不分页查询并捕获其 SELECT 语句。
// 与 SelectPageList 不同，这里 Find 无 Count 前置，DryRun 也能拿到完整 SELECT。
func captureListSQL(t *testing.T, q bo.SysClientQueryBo, limit int) string {
	t.Helper()
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_list", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewClientRepository(db).SelectList(t.Context(), q, limit); err != nil {
		t.Fatalf("SelectList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql
}

// TestSelectListWhereConditions 四个过滤条件全传时都应落到 WHERE，且保留逻辑删除过滤。
func TestSelectListWhereConditions(t *testing.T) {
	q := bo.SysClientQueryBo{
		ClientID:     "e5cd7e4891bf95d1d19206ce24a7b32e",
		ClientKey:    "pc",
		ClientSecret: "pc123",
		Status:       "0",
	}
	got := captureListSQL(t, q, 0)

	for _, want := range []string{
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

// TestSelectListSkipsEmptyConditions 空串条件不参与筛选（eqIfText 语义）。
func TestSelectListSkipsEmptyConditions(t *testing.T) {
	got := captureListSQL(t, bo.SysClientQueryBo{}, 0)

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

// TestSelectListAppliesLimit limit>0 才拼 LIMIT，0 与 -1 表示不限制。
func TestSelectListAppliesLimit(t *testing.T) {
	if got := captureListSQL(t, bo.SysClientQueryBo{}, 100001); !strings.Contains(got, "LIMIT") {
		t.Errorf("limit=100001 时应带 LIMIT, SQL = %s", got)
	}
	if got := captureListSQL(t, bo.SysClientQueryBo{}, 0); strings.Contains(got, "LIMIT") {
		t.Errorf("limit=0 不应带 LIMIT, SQL = %s", got)
	}
	if got := captureListSQL(t, bo.SysClientQueryBo{}, -1); strings.Contains(got, "LIMIT") {
		t.Errorf("limit=-1 不应带 LIMIT, SQL = %s", got)
	}
}

// TestSelectListOrdersByID 不分页查询无调用方排序一说，固定按主键升序，输出顺序稳定。
func TestSelectListOrdersByID(t *testing.T) {
	got := captureListSQL(t, bo.SysClientQueryBo{}, 0)
	if !strings.Contains(got, "ORDER BY") {
		t.Errorf("SQL = %s\n应带 ORDER BY", got)
	}
	if !strings.Contains(got, "`id`") && !strings.Contains(got, "id") {
		t.Errorf("SQL = %s\n应按 id 排序", got)
	}
}

// TestApplyClientQuerySharedByBothPaths 分页与列表两条路径必须生成一致的 WHERE。
// 这是 applyClientQuery 存在的全部理由：改过滤逻辑时只改一处，两条路径同步生效。
func TestApplyClientQuerySharedByBothPaths(t *testing.T) {
	q := bo.SysClientQueryBo{
		ClientID:     "e5cd7e4891bf95d1d19206ce24a7b32e",
		ClientKey:    "pc",
		ClientSecret: "pc123",
		Status:       "0",
	}

	// 列表路径的 SELECT；分页路径取 COUNT 语句。
	listSQL := captureListSQL(t, q, 0)
	pageSQL := captureSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]

	// 关键：四个条件的 WHERE 片段必须逐字出现在两条 SQL 里，不允许一边漏一个。
	for _, want := range []string{"client_id = ?", "client_key = ?", "client_secret = ?", "status = ?"} {
		if !strings.Contains(listSQL, want) {
			t.Errorf("列表 SQL 缺少 %s: %s", want, listSQL)
		}
		if !strings.Contains(pageSQL, want) {
			t.Errorf("分页 SQL 缺少 %s: %s", want, pageSQL)
		}
	}
}
