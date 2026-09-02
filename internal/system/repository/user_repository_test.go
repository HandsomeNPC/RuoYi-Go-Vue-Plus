package repository

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureUserSQL 在 DryRun 模式下执行用户分页查询并捕获途中所有 SQL。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句，
// 但 WHERE 子句在 COUNT 里完整出现，足以断言过滤条件。
// 与 client 测试同款的捕获写法（包内 dryClientDB 复用）。
func captureUserSQL(t *testing.T, q bo.SysUserQueryBo, page pkgrepo.PageQuery) []string {
	t.Helper()
	db := dryClientDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:captureUser", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewUserRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	return sqls
}

// TestUserSelectPageListAllConditions 全部条件传值时都应落到 WHERE，并保留逻辑删除过滤。
func TestUserSelectPageListAllConditions(t *testing.T) {
	q := bo.SysUserQueryBo{
		UserName:       "admin",
		NickName:       "管理员",
		Status:         "0",
		PhoneNumber:    "138",
		UserID:         1,
		UserIDs:        "10,11",
		ExcludeUserIDs: "20,21",
		// DeptIDs 由 service 按 DeptID 解析后填入，repo 只按集合 IN 过滤，此处直接给模拟结果。
		DeptIDs:   []int64{100, 101},
		BeginTime: "2024-01-01 00:00:00",
		EndTime:   "2024-12-31 23:59:59",
	}
	got := captureUserSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})[0]

	for _, want := range []string{
		"count(*)",
		"FROM `sys_user`",
		"sys_user.user_name LIKE ?",
		"sys_user.nick_name LIKE ?",
		"sys_user.status = ?",
		"sys_user.phone_number LIKE ?",
		"sys_user.user_id = ?",
		"sys_user.user_id IN", // GORM 把 IN ? 展开成 IN (?,?)，只断言子串避开占位符数量
		"sys_user.user_id NOT IN",
		"sys_user.dept_id IN",
		"sys_user.create_time BETWEEN ? AND ?",
		"del_flag", // 逻辑删除过滤由 LogicDelete 注入
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL = %s\n应包含 %s", got, want)
		}
	}
}

// TestUserSelectPageListSkipsEmptyConditions 空串/零值条件不参与筛选（likeIfText/eqIfPresent 语义）。
func TestUserSelectPageListSkipsEmptyConditions(t *testing.T) {
	got := captureUserSQL(t, bo.SysUserQueryBo{}, pkgrepo.PageQuery{})[0]

	for _, unwanted := range []string{
		"sys_user.user_name LIKE ?",
		"sys_user.nick_name LIKE ?",
		"sys_user.status = ?",
		"sys_user.phone_number LIKE ?",
		"sys_user.user_id = ?",
		"sys_user.user_id IN ?",
		"sys_user.user_id NOT IN ?",
		"sys_user.dept_id IN ?",
		"sys_user.create_time BETWEEN",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("SQL = %s\n不应包含 %s", got, unwanted)
		}
	}
	// 逻辑删除条件与表名仍须在。
	if !strings.Contains(got, "del_flag") || !strings.Contains(got, "FROM `sys_user`") {
		t.Errorf("SQL = %s\n应保留 del_flag 与 sys_user", got)
	}
}

// TestUserSelectPageListSingleTimeEndNoFilter 只给一端时间不筛（对齐 Java betweenParams 的两端同时给才生效）。
func TestUserSelectPageListSingleTimeEndNoFilter(t *testing.T) {
	got := captureUserSQL(t, bo.SysUserQueryBo{
		BeginTime: "2024-01-01 00:00:00",
	}, pkgrepo.PageQuery{})[0]
	if strings.Contains(got, "sys_user.create_time BETWEEN") {
		t.Errorf("只给 BeginTime 不该生成 BETWEEN，SQL = %s", got)
	}
}
