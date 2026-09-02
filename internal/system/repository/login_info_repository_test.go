package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureLoginInfoSQL 执行分页查询并捕获 SQL 与绑定变量。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureLoginInfoSQL(t *testing.T, q bo.SysLoginInfoQueryBo,
	page pkgrepo.PageQuery) (string, []any) {

	t.Helper()
	db := dryClientDB(t)
	var sql string
	var vars []any
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture:logininfo", func(tx *gorm.DB) {
			if sql == "" {
				sql = tx.Statement.SQL.String()
				vars = tx.Statement.Vars
			}
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewLoginInfoRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestLoginInfoSelectPageQueryAll 筛选条件全传时都落到 WHERE：
// ipaddr/userName 走 LIKE（含 % 包裹）、status 走 =、login_time 走闭区间 BETWEEN。
func TestLoginInfoSelectPageQueryAll(t *testing.T) {
	q := bo.SysLoginInfoQueryBo{
		IPAddr: "192", UserName: "admin", Status: "1",
		BeginTime: "2026-01-01", EndTime: "2026-02-01",
	}
	sql, vars := captureLoginInfoSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, want := range []string{
		"ipaddr LIKE",
		"user_name LIKE",
		"status =",
		"login_time BETWEEN",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q\nSQL: %s", want, sql)
		}
	}
	// 三个过滤值 + 区间两端，共 5 个绑定变量（LIKE 是 %val% 整串一个变量）。
	if len(vars) != 5 {
		t.Errorf("绑定变量数 = %d, 期望 5\nvars: %v", len(vars), vars)
	}
}

// TestLoginInfoSelectPageQueryEmpty 空串一概不筛：WHERE 段不含任何业务过滤列。
// 对齐 Java likeIfText/eqIfText/betweenParams 在入参为空时的不筛语义。
func TestLoginInfoSelectPageQueryEmpty(t *testing.T) {
	sql, _ := captureLoginInfoSQL(t, bo.SysLoginInfoQueryBo{},
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, unwanted := range []string{
		"ipaddr LIKE", "user_name LIKE", "status =", "login_time BETWEEN",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("空入参不应出现 %q\nSQL: %s", unwanted, sql)
		}
	}
}

// TestLoginInfoSelectPageQueryHalfDate 只给一端时间不筛 login_time，
// 对齐 Java betweenParams 的 begin != null && end != null 才生效。
func TestLoginInfoSelectPageQueryHalfDate(t *testing.T) {
	q := bo.SysLoginInfoQueryBo{BeginTime: "2026-01-01"} // 只有 begin
	sql, _ := captureLoginInfoSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	if strings.Contains(sql, "login_time BETWEEN") {
		t.Errorf("只给一端时间不应筛 login_time\nSQL: %s", sql)
	}
}

// TestLoginInfoSelectPageQueryLikeEscape LIKE 元字符按字面量转义，搜 "%" 不会命中全表。
func TestLoginInfoSelectPageQueryLikeEscape(t *testing.T) {
	q := bo.SysLoginInfoQueryBo{UserName: "%admin_"}
	_, vars := captureLoginInfoSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	// 转义后应为 %\%admin\_%（首尾的 % 是 LIKE 通配，中间的 \% \_ 是字面量）。
	want := "%\\%admin\\_%"
	if len(vars) != 1 || vars[0] != want {
		t.Errorf("LIKE 转义 = %v, 期望 %q", vars, want)
	}
}
