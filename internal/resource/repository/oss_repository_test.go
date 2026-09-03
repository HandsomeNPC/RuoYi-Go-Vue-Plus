package repository

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"ruoyi-go-vue-plus/internal/resource/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// dryDB DryRun 模式的 gorm 实例，只拼 SQL 不连库。
// internal/system/repository 里的同名辅助是那个包的测试代码，跨包取不到，故本包自备。
func dryDB(t *testing.T) *gorm.DB {
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

// captureOssSQL 执行分页查询并捕获途中所有 SQL。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT 语句。
func captureOssSQL(t *testing.T, q bo.SysOssQueryBo, page pkgrepo.PageQuery) string {
	t.Helper()
	db := dryDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	if _, err := NewOssRepository(db).SelectPageList(t.Context(), q, page); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	return sqls[0]
}

// TestApplyOssQueryAllConditions 条件全传时都应落到 WHERE。
func TestApplyOssQueryAllConditions(t *testing.T) {
	q := bo.SysOssQueryBo{
		FileName:        "2026/01/01/abc.png",
		OriginalName:    "头像.png",
		FileSuffix:      ".png",
		URL:             "http://127.0.0.1:9000/ruoyi/a.png",
		Service:         "minio",
		CreateBy:        1,
		BeginCreateTime: "2026-01-01 00:00:00",
		EndCreateTime:   "2026-01-31 23:59:59",
	}
	got := captureOssSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	for _, want := range []string{
		"count(*)",
		"FROM `sys_oss`",
		"file_name LIKE ?",
		"original_name LIKE ?",
		"file_suffix = ?",
		"url = ?",
		"service = ?",
		"create_by = ?",
		"create_time BETWEEN ? AND ?",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL 缺少 %q\n实际: %s", want, got)
		}
	}
}

// TestApplyOssQueryEmptyConditions 空值一概不落 WHERE（对齐 Java likeIfText/eqIfText 语义）。
func TestApplyOssQueryEmptyConditions(t *testing.T) {
	got := captureOssSQL(t, bo.SysOssQueryBo{}, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})

	if strings.Contains(got, "WHERE") {
		t.Errorf("无条件时不应出现 WHERE\n实际: %s", got)
	}
	// create_by 是 int64，0 必须被当成"未指定"而非"上传人为 0"。
	if strings.Contains(got, "create_by") {
		t.Errorf("CreateBy=0 不应落 WHERE\n实际: %s", got)
	}
}

// TestApplyOssQueryTimeRangeNeedsBothEnds 时间区间只给一端时不筛。
// 只给一端就筛会让前端清空半个日期框时结果突变。
func TestApplyOssQueryTimeRangeNeedsBothEnds(t *testing.T) {
	for _, q := range []bo.SysOssQueryBo{
		{BeginCreateTime: "2026-01-01 00:00:00"},
		{EndCreateTime: "2026-01-31 23:59:59"},
	} {
		got := captureOssSQL(t, q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10})
		if strings.Contains(got, "create_time") {
			t.Errorf("仅给区间一端时不应筛 create_time\n实际: %s", got)
		}
	}
}

// TestApplyOssConfigQuery 配置查询：配置键与状态走 eq，桶名走 like。
func TestApplyOssConfigQuery(t *testing.T) {
	db := dryDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册 callback 失败: %v", err)
	}

	q := bo.SysOssConfigQueryBo{ConfigKey: "minio", BucketName: "ruoyi", Status: "Y"}
	if _, err := NewOssConfigRepository(db).
		SelectPageList(t.Context(), q, pkgrepo.PageQuery{PageNum: 1, PageSize: 10}); err != nil {
		t.Fatalf("SelectPageList: %v", err)
	}
	if len(sqls) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	got := sqls[0]

	for _, want := range []string{
		"FROM `sys_oss_config`",
		"config_key = ?",
		"bucket_name LIKE ?",
		"status = ?",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SQL 缺少 %q\n实际: %s", want, got)
		}
	}
}

// TestEscapeLike LIKE 元字符按字面量匹配，不转义的话搜 % 会命中全表。
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"a%b": `a\%b`,
		"a_b": `a\_b`,
		`a\b`: `a\\b`,
		"正常":  "正常",
		`%_\`: `\%\_\\`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
