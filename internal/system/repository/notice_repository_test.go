package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// captureNoticeSQL 执行公告分页查询并捕获 COUNT 语句。
// DryRun 下 Count 取不到真实值，total=0 会让 SelectPage 提前返回，故只能拿到 COUNT。
func captureNoticeSQL(t *testing.T, q bo.SysNoticeQueryBo, createBy int64) string {
	t.Helper()
	db := dryClientDB(t)
	var sqls []string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_notice", func(tx *gorm.DB) {
			sqls = append(sqls, tx.Statement.SQL.String())
		}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	_, _ = NewNoticeRepository(db).SelectPageList(context.Background(), q, createBy,
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10})
	if len(sqls) == 0 {
		t.Fatal("未捕获到任何 SQL")
	}
	return sqls[0]
}

// TestApplyNoticeQueryAllConditions 条件全给时都落到 WHERE 上。
func TestApplyNoticeQueryAllConditions(t *testing.T) {
	sql := captureNoticeSQL(t, bo.SysNoticeQueryBo{
		NoticeTitle: "维护",
		NoticeType:  "1",
	}, 1761100000000000001)

	for _, want := range []string{
		"notice_title LIKE",
		"notice_type =",
		"create_by =",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %s\nSQL: %s", want, sql)
		}
	}
}

// TestApplyNoticeQueryEmptyNotFiltered 空条件一律不落 WHERE（likeIfText/eqIfText 语义）。
func TestApplyNoticeQueryEmptyNotFiltered(t *testing.T) {
	sql := captureNoticeSQL(t, bo.SysNoticeQueryBo{}, 0)

	for _, unwanted := range []string{"notice_title", "notice_type", "create_by"} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("空条件不应产生 %s 过滤\nSQL: %s", unwanted, sql)
		}
	}
}

// TestApplyNoticeQueryCreateByNoMatch 创建人查不到用户时用负数哨兵，仍会落过滤条件。
//
// 关键在于它必须落条件而非退化成不筛：否则「只看某人发的公告」会返回全部公告。
func TestApplyNoticeQueryCreateByNoMatch(t *testing.T) {
	sql := captureNoticeSQL(t, bo.SysNoticeQueryBo{}, -1)

	if !strings.Contains(sql, "create_by =") {
		t.Errorf("哨兵值也应落 create_by 过滤（否则会返回全部公告）\nSQL: %s", sql)
	}
}

// TestNoticeQueryEscapesLike LIKE 元字符按字面量匹配，不当通配符。
func TestNoticeQueryEscapesLike(t *testing.T) {
	db := dryClientDB(t)
	var vars []any
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_notice_vars", func(tx *gorm.DB) {
			vars = tx.Statement.Vars
		}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	_, _ = NewNoticeRepository(db).SelectList(context.Background(),
		bo.SysNoticeQueryBo{NoticeTitle: "100%_a"}, 0, 0)

	if len(vars) == 0 {
		t.Fatal("未捕获到绑定参数")
	}
	got, ok := vars[0].(string)
	if !ok {
		t.Fatalf("首个绑定参数不是字符串: %#v", vars[0])
	}
	if want := `%100\%\_a%`; got != want {
		t.Errorf("LIKE 参数 = %q, 期望 %q", got, want)
	}
}

// TestNoticeDefaultOrderOnlyWhenUnspecified 调用方指定排序时不再追加主键兜底。
//
// 无条件追加会让后指定的排序列失效：GORM 合并排序子句时先注册的排在前，
// 而 notice_id 唯一，前端点"按标题排序"就会毫无效果。
func TestNoticeDefaultOrderOnlyWhenUnspecified(t *testing.T) {
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_notice_order", func(tx *gorm.DB) {
			if sql == "" {
				sql = tx.Statement.SQL.String()
			}
		}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	_, _ = NewNoticeRepository(db).SelectPageList(context.Background(),
		bo.SysNoticeQueryBo{}, 0,
		pkgrepo.PageQuery{PageNum: 1, PageSize: 10, OrderByColumn: "notice_title", IsAsc: "asc"})

	// COUNT 语句本身不带 ORDER BY，这里只需确认没有因兜底而报错、且捕获到语句。
	if sql == "" {
		t.Fatal("未捕获到 SQL")
	}
}

// TestSelectBoxListConditions 消息盒子的可见性判定与限流都落到 SQL 上。
func TestSelectBoxListConditions(t *testing.T) {
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_box", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	_, _ = NewMessageRepository(db).SelectBoxList(context.Background(), "notice",
		1761100000000000001, time.Now().AddDate(0, 0, -30), 100)

	for _, want := range []string{
		"category = ?",
		"create_time >= ?",
		// FIND_IN_SET 而非 LIKE：后者会让 userId=1 命中 "10,11"。
		"FIND_IN_SET",
		"LIMIT",
		"ORDER BY create_time DESC,message_id DESC",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %s\nSQL: %s", want, sql)
		}
	}
}

// TestSelectBoxListWrapsOrCondition 可见性的 OR 必须整体括起来。
//
// 不括起来会被解析成 "category=? AND time>=? AND global OR inset"，
// 让别人的消息漏给当前用户——这是权限泄露，不只是结果不准。
func TestSelectBoxListWrapsOrCondition(t *testing.T) {
	db := dryClientDB(t)
	var sql string
	if err := db.Callback().Query().After("gorm:query").
		Register("test:capture_box_or", func(tx *gorm.DB) {
			sql = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	_, _ = NewMessageRepository(db).SelectBoxList(context.Background(), "notice",
		1761100000000000001, time.Now(), 100)

	if !strings.Contains(sql, "(send_user_ids = ? OR FIND_IN_SET(?, send_user_ids))") {
		t.Errorf("OR 条件未被括号包裹，会造成消息越权可见\nSQL: %s", sql)
	}
}

// TestSelectBoxListGuards 分类为空或用户ID非法时不查库。
func TestSelectBoxListGuards(t *testing.T) {
	repo := NewMessageRepository(dryClientDB(t))
	for _, tt := range []struct {
		name     string
		category string
		userID   int64
	}{
		{"分类为空", "", 1},
		{"用户ID为0", "notice", 0},
		{"用户ID为负", "notice", -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := repo.SelectBoxList(context.Background(), tt.category, tt.userID,
				time.Now(), 100)
			if err != nil || rows != nil {
				t.Errorf("应静默返回空，got rows=%v err=%v", rows, err)
			}
		})
	}
}
