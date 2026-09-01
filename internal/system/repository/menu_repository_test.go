package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
)

// captureMenuSQL 执行列表查询并捕获 SQL 与绑定变量。
// byUser 决定走超管全量分支还是用户联表分支。
func captureMenuSQL(t *testing.T, q bo.SysMenuQueryBo, byUser bool) (string, []any) {
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

	repo := NewMenuRepository(db)
	var err error
	if byUser {
		_, err = repo.SelectListByUserId(t.Context(), q, 1761100000000000001)
	} else {
		_, err = repo.SelectList(t.Context(), q)
	}
	if err != nil {
		t.Fatalf("查询菜单列表: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestMenuQueryConditions 五个条件全传时都应落到 WHERE：
// 名称走 LIKE，其余走 =（对齐 Java likeIfText/eqIfText/eqIfPresent）。
func TestMenuQueryConditions(t *testing.T) {
	sql, vars := captureMenuSQL(t, bo.SysMenuQueryBo{
		MenuName: "用户",
		Visible:  "0",
		Status:   "0",
		MenuType: "C",
		ParentID: 1761200000000000001,
	}, false)

	for _, want := range []string{
		"menu_name LIKE ?",
		"visible = ?",
		"status = ?",
		"menu_type = ?",
		"parent_id = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
	// 构树前提：service 依赖同级节点已按 order_num 有序。
	if !strings.Contains(sql, "ORDER BY parent_id,order_num") {
		t.Errorf("排序不符:\n%s", sql)
	}
	if got, want := len(vars), 5; got != want {
		t.Errorf("绑定变量数 = %d, 期望 %d: %v", got, want, vars)
	}
}

// TestMenuQueryEmptyConditions 条件全空时不该凭空多出 WHERE 列。
func TestMenuQueryEmptyConditions(t *testing.T) {
	sql, vars := captureMenuSQL(t, bo.SysMenuQueryBo{}, false)

	for _, unwanted := range []string{
		"menu_name LIKE", "visible = ?", "status = ?", "menu_type = ?", "parent_id = ?",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("SQL 不应含 %q:\n%s", unwanted, sql)
		}
	}
	if len(vars) != 0 {
		t.Errorf("不应有绑定变量: %v", vars)
	}
}

// TestMenuQueryEscapesLike 菜单名里的 LIKE 元字符须按字面量匹配，
// 不转义的话搜 "%" 会命中全表。
func TestMenuQueryEscapesLike(t *testing.T) {
	_, vars := captureMenuSQL(t, bo.SysMenuQueryBo{MenuName: "100%_x"}, false)

	found := false
	for _, v := range vars {
		if s, ok := v.(string); ok && strings.Contains(s, `100\%\_x`) {
			found = true
		}
	}
	if !found {
		t.Errorf("LIKE 元字符未转义: %v", vars)
	}
}

// TestMenuQueryByUserIDQualifiesColumns 用户分支的筛选列必须带 sys_menu. 前缀——
// 三张联表都有 status 列，不限定会撞上 sr.status 而报歧义。
func TestMenuQueryByUserIDQualifiesColumns(t *testing.T) {
	sql, _ := captureMenuSQL(t, bo.SysMenuQueryBo{
		MenuName: "用户",
		Status:   "0",
		Visible:  "0",
		MenuType: "C",
		ParentID: 1761200000000000001,
	}, true)

	for _, want := range []string{
		"sys_menu.menu_name LIKE ?",
		"sys_menu.visible = ?",
		"sys_menu.status = ?",
		"sys_menu.menu_type = ?",
		"sys_menu.parent_id = ?",
		"sr.status = ?", // 角色状态过滤仍在
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
	// 裸 status = ? 会在联表下歧义，必须一个都不剩。
	if strings.Contains(sql, "WHERE status = ?") || strings.Contains(sql, "AND status = ?") {
		t.Errorf("联表查询出现未限定的 status 列:\n%s", sql)
	}
}

// TestMenuExistsByParentIDsExcludesSelf 级联删除时父子可能同批提交，
// 子菜单会随父一起删掉，不该算"存在子菜单"——故须 NOT IN 自身。
func TestMenuExistsByParentIDsExcludesSelf(t *testing.T) {
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
	if _, err := NewMenuRepository(db).ExistsByParentIDs(t.Context(),
		[]int64{1761200000000000001, 1761200000000000002}); err != nil {
		t.Fatalf("ExistsByParentIDs: %v", err)
	}

	if !strings.Contains(sql, "parent_id IN") {
		t.Errorf("SQL 缺少父级过滤:\n%s", sql)
	}
	if !strings.Contains(sql, "menu_id NOT IN") {
		t.Errorf("SQL 缺少排除自身:\n%s", sql)
	}
}

// TestMenuUniqueCheckExcludesSelf 名称判重须按 name + parent 两列，并能排除自身，
// 否则修改时把名称改回原样会被误判成冲突。
func TestMenuUniqueCheckExcludesSelf(t *testing.T) {
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
	if _, err := NewMenuRepository(db).ExistsByMenuName(t.Context(), "用户管理",
		1761200000000000001, 1761200000000000009); err != nil {
		t.Fatalf("ExistsByMenuName: %v", err)
	}

	for _, want := range []string{"menu_name = ?", "parent_id = ?", "menu_id <> ?"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q:\n%s", want, sql)
		}
	}
}

// TestMenuRouteConflictCandidatesScope 候选集只在目录/菜单里找，且 path 与 routeName
// 两值都要匹配——按钮无路由，混进来会造出无谓的冲突。
func TestMenuRouteConflictCandidatesScope(t *testing.T) {
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
	if _, err := NewMenuRepository(db).SelectRouteConflictCandidates(t.Context(),
		"user", "User"); err != nil {
		t.Fatalf("SelectRouteConflictCandidates: %v", err)
	}

	if !strings.Contains(sql, "menu_type IN") {
		t.Errorf("SQL 缺少类型过滤:\n%s", sql)
	}
	if !strings.Contains(sql, "path = ? OR path = ?") {
		t.Errorf("SQL 缺少 path/routeName 双值匹配:\n%s", sql)
	}
	// M、C 两个类型值 + path、routeName 两个值。
	if got, want := len(vars), 4; got != want {
		t.Errorf("绑定变量数 = %d, 期望 %d: %v", got, want, vars)
	}
}
