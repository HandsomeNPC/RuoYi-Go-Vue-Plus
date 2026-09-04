package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureOperLogSQL 执行分页查询并捕获 SQL 与绑定变量。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureOperLogSQL(t *testing.T, q bo.SysOperLogQueryBo,
	page pkgrepo.PageQuery) (string, []any) {

	t.Helper()
	db := dryClientDB(t)
	var sql string
	var vars []any
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture:operlog", func(tx *gorm.DB) {
			if sql == "" {
				sql = tx.Statement.SQL.String()
				vars = tx.Statement.Vars
			}
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewOperLogRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
	return sql, vars
}

// TestOperLogSelectPageQueryAll 筛选条件全传时都落到 WHERE：
// operIp/title/operName/browser/os 走 LIKE、businessType/status/userId/deptId/clientKey/deviceType 走 =、
// businessTypes 走 IN、oper_time 走闭区间 BETWEEN。
func TestOperLogSelectPageQueryAll(t *testing.T) {
	q := bo.SysOperLogQueryBo{
		OperIP: "192", Title: "用户", BusinessType: 1, BusinessTypes: []int{1, 2},
		Status: "1", OperName: "admin", UserID: 100, DeptID: 200,
		ClientKey: "e5cd7", DeviceType: "pc", Browser: "Chrome", OS: "Windows",
		BeginTime: "2026-01-01", EndTime: "2026-02-01",
	}
	sql, vars := captureOperLogSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, want := range []string{
		"oper_ip LIKE", "title LIKE", "business_type =",
		"business_type IN", "status =", "oper_name LIKE",
		"user_id =", "dept_id =", "client_key =", "device_type =",
		"browser LIKE", "os LIKE", "oper_time BETWEEN",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q\nSQL: %s", want, sql)
		}
	}
	// 5 个 LIKE + 5 个 eq（businessType/status/userId/deptId/clientKey/deviceType 去重共 6 个 eq，
	// 但 business_type 同时有 = 与 IN 两条子句）+ 1 个 IN + 区间两端，绑定量随驱动展开。
	if len(vars) == 0 {
		t.Errorf("未捕获到绑定变量")
	}
}

// TestOperLogSelectPageQueryEmpty 空串/零值一概不筛：WHERE 段不含任何业务过滤列。
// 入参为空时不落 WHERE 条件。
func TestOperLogSelectPageQueryEmpty(t *testing.T) {
	sql, _ := captureOperLogSQL(t, bo.SysOperLogQueryBo{},
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, unwanted := range []string{
		"oper_ip LIKE", "title LIKE", "business_type =",
		"business_type IN", "status =", "oper_name LIKE",
		"user_id =", "dept_id =", "client_key =", "device_type =",
		"browser LIKE", "os LIKE", "oper_time BETWEEN",
	} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("空入参不应出现 %q\nSQL: %s", unwanted, sql)
		}
	}
}

// TestOperLogBusinessTypeZeroNotFilter 单值 businessType=0 不参与过滤，
// businessType=0 是"其他"，要筛"其他"得走 businessTypes 集合。
func TestOperLogBusinessTypeZeroNotFilter(t *testing.T) {
	q := bo.SysOperLogQueryBo{BusinessType: 0}
	sql, _ := captureOperLogSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})
	if strings.Contains(sql, "business_type =") {
		t.Errorf("businessType=0 不应筛\nSQL: %s", sql)
	}
}

// TestOperLogSelectPageQueryHalfDate 只给一端时间不筛 oper_time，
// 两端同时给才生效。
func TestOperLogSelectPageQueryHalfDate(t *testing.T) {
	q := bo.SysOperLogQueryBo{BeginTime: "2026-01-01"} // 只有 begin
	sql, _ := captureOperLogSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	if strings.Contains(sql, "oper_time BETWEEN") {
		t.Errorf("只给一端时间不应筛 oper_time\nSQL: %s", sql)
	}
}

// TestOperLogSelectPageQueryLikeEscape LIKE 元字符按字面量转义，搜 "%" 不会命中全表。
func TestOperLogSelectPageQueryLikeEscape(t *testing.T) {
	q := bo.SysOperLogQueryBo{Title: "%用户_"}
	_, vars := captureOperLogSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	// 转义后应为 %\%用户\_%（首尾的 % 是 LIKE 通配，中间的 % \_ 是字面量）。
	want := "%\\%用户\\_%"
	if len(vars) != 1 || vars[0] != want {
		t.Errorf("LIKE 转义 = %v, 期望 %q", vars, want)
	}
}
