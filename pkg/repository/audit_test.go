package repository

import (
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// auditFixture 带全部五个审计列的实体。
type auditFixture struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	Name       string     `gorm:"column:name"`
	CreateDept int64      `gorm:"column:create_dept"`
	CreateBy   int64      `gorm:"column:create_by"`
	CreateTime *time.Time `gorm:"column:create_time"`
	UpdateBy   int64      `gorm:"column:update_by"`
	UpdateTime *time.Time `gorm:"column:update_time"`
}

func (auditFixture) TableName() string { return "audit_fixture" }

// plainFixture 不含任何审计列，用于确认回调不误伤。
type plainFixture struct {
	ID   int64  `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (plainFixture) TableName() string { return "plain_fixture" }

// dryAuditDB DryRun 实例并注册审计回调，只拼 SQL 不连库。
func dryAuditDB(t *testing.T) *gorm.DB {
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
	if err := RegisterAuditCallbacks(db); err != nil {
		t.Fatalf("注册审计回调失败: %v", err)
	}
	return db
}

// TestAuditInsertFillNoLoginUser ctx 无操作者时回落 -1（对齐 Java DEFAULT_USER_ID）。
func TestAuditInsertFillNoLoginUser(t *testing.T) {
	row := &auditFixture{ID: 1, Name: "x"}
	dryAuditDB(t).Create(row)

	if row.CreateBy != AuditDefaultUserID || row.UpdateBy != AuditDefaultUserID {
		t.Errorf("CreateBy/UpdateBy = %d/%d, 期望均为 %d",
			row.CreateBy, row.UpdateBy, AuditDefaultUserID)
	}
	if row.CreateDept != AuditDefaultUserID {
		t.Errorf("CreateDept = %d, 期望 %d", row.CreateDept, AuditDefaultUserID)
	}
	if row.CreateTime == nil || row.UpdateTime == nil {
		t.Fatal("CreateTime/UpdateTime 应被填充")
	}
	if !row.CreateTime.Equal(*row.UpdateTime) {
		t.Errorf("插入时 CreateTime(%v) 应与 UpdateTime(%v) 相等", row.CreateTime, row.UpdateTime)
	}
}

// TestAuditInsertFillWithAuditUser ctx 带操作者时按其 userID/deptID 填充。
func TestAuditInsertFillWithAuditUser(t *testing.T) {
	const userID, deptID int64 = 1761100000000000001, 1761000000000000103

	row := &auditFixture{ID: 2, Name: "y"}
	ctx := WithAuditUser(t.Context(), AuditUser{UserID: userID, DeptID: deptID})
	dryAuditDB(t).WithContext(ctx).Create(row)

	if row.CreateBy != userID || row.UpdateBy != userID {
		t.Errorf("CreateBy/UpdateBy = %d/%d, 期望均为 %d", row.CreateBy, row.UpdateBy, userID)
	}
	if row.CreateDept != deptID {
		t.Errorf("CreateDept = %d, 期望 %d", row.CreateDept, deptID)
	}
}

// TestAuditInsertFillKeepsPresetCreateTime 预设 CreateTime 须保留，且 UpdateTime 与之对齐。
func TestAuditInsertFillKeepsPresetCreateTime(t *testing.T) {
	preset := time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &auditFixture{ID: 3, Name: "z", CreateTime: &preset}
	dryAuditDB(t).Create(row)

	if !row.CreateTime.Equal(preset) {
		t.Errorf("CreateTime = %v, 期望保留预设值 %v", row.CreateTime, preset)
	}
	if row.UpdateTime == nil || !row.UpdateTime.Equal(preset) {
		t.Errorf("UpdateTime = %v, 期望与预设 CreateTime 对齐", row.UpdateTime)
	}
}

// TestAuditInsertFillKeepsPresetCreateBy 已指定创建人时三个人员字段整体不动
// （对齐 Java if (createBy == null) 的整块守卫）。
func TestAuditInsertFillKeepsPresetCreateBy(t *testing.T) {
	const presetBy int64 = 999

	row := &auditFixture{ID: 4, Name: "w", CreateBy: presetBy}
	ctx := WithAuditUser(t.Context(), AuditUser{UserID: 111, DeptID: 222})
	dryAuditDB(t).WithContext(ctx).Create(row)

	if row.CreateBy != presetBy {
		t.Errorf("CreateBy = %d, 期望保留预设值 %d", row.CreateBy, presetBy)
	}
	if row.UpdateBy != 0 || row.CreateDept != 0 {
		t.Errorf("UpdateBy/CreateDept = %d/%d, 预设 CreateBy 时二者应保持零值",
			row.UpdateBy, row.CreateDept)
	}
}

// TestAuditInsertFillBatchKeepsPerRowCreateTime 批量插入时各行预设 CreateTime 不得互相冲掉。
// 这一条守的是「不能用 stmt.SetColumn 填插入路径」——它在回调期会拿同一个值遍历整个 slice。
func TestAuditInsertFillBatchKeepsPerRowCreateTime(t *testing.T) {
	t1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2022, 2, 2, 0, 0, 0, 0, time.UTC)
	rows := []*auditFixture{
		{ID: 5, Name: "a", CreateTime: &t1},
		{ID: 6, Name: "b", CreateTime: &t2},
		{ID: 7, Name: "c"}, // 未预设，应取当前时间
	}
	dryAuditDB(t).Create(rows)

	if !rows[0].CreateTime.Equal(t1) {
		t.Errorf("第 1 行 CreateTime = %v, 期望 %v", rows[0].CreateTime, t1)
	}
	if !rows[1].CreateTime.Equal(t2) {
		t.Errorf("第 2 行 CreateTime = %v, 期望 %v(不应被第 1 行冲掉)", rows[1].CreateTime, t2)
	}
	if rows[2].CreateTime == nil {
		t.Fatal("第 3 行 CreateTime 应被填充")
	}
	if rows[2].CreateTime.Equal(t1) || rows[2].CreateTime.Equal(t2) {
		t.Errorf("第 3 行 CreateTime = %v, 不应取到其他行的预设值", rows[2].CreateTime)
	}
	for i, r := range rows {
		if !r.CreateTime.Equal(*r.UpdateTime) {
			t.Errorf("第 %d 行 CreateTime(%v) 应与 UpdateTime(%v) 相等",
				i+1, r.CreateTime, r.UpdateTime)
		}
	}
}

// TestAuditUpdateFill 更新时无条件刷新 update_by/update_time，不动 create_*。
func TestAuditUpdateFill(t *testing.T) {
	const userID int64 = 1761100000000000001

	row := &auditFixture{ID: 8, Name: "u"}
	ctx := WithAuditUser(t.Context(), AuditUser{UserID: userID, DeptID: 333})
	dryAuditDB(t).WithContext(ctx).Model(row).Update("name", "u2")

	if row.UpdateBy != userID {
		t.Errorf("UpdateBy = %d, 期望 %d", row.UpdateBy, userID)
	}
	if row.UpdateTime == nil {
		t.Error("UpdateTime 应被填充")
	}
	if row.CreateBy != 0 || row.CreateTime != nil {
		t.Errorf("更新不应触碰 CreateBy(%d)/CreateTime(%v)", row.CreateBy, row.CreateTime)
	}
}

// TestAuditFillSkipsTableWithoutAuditColumns 无审计列的表不受影响（不 panic、不误加列）。
func TestAuditFillSkipsTableWithoutAuditColumns(t *testing.T) {
	db := dryAuditDB(t)

	row := &plainFixture{ID: 9, Name: "p"}
	if err := db.Create(row).Statement.Error; err != nil {
		t.Fatalf("插入无审计列的表失败: %v", err)
	}
	sql := db.Session(&gorm.Session{DryRun: true}).Create(&plainFixture{ID: 10}).Statement.SQL.String()
	for _, col := range []string{colCreateBy, colCreateTime, colUpdateBy, colUpdateTime, colCreateDept} {
		if contains(sql, col) {
			t.Errorf("SQL = %s\n不应出现审计列 %s", sql, col)
		}
	}
}

// TestAuditUserFrom ctx 未写入时返回 ok=false。
func TestAuditUserFrom(t *testing.T) {
	if _, ok := AuditUserFrom(t.Context()); ok {
		t.Error("空 ctx 应返回 ok=false")
	}

	want := AuditUser{UserID: 7, DeptID: 8}
	got, ok := AuditUserFrom(WithAuditUser(t.Context(), want))
	if !ok || got != want {
		t.Errorf("AuditUserFrom = %+v(ok=%v), 期望 %+v(ok=true)", got, ok, want)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
