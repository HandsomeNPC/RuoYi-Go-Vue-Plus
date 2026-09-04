package repository

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// captureSocialSQL 执行一次 social 仓储操作并捕获途中的 SQL 及其绑定参数。
// 复用同目录 client_repository_test.go 的 dryClientDB（test-only，跨包取不到）。
//
// 查询与删除各挂一个回调：GORM 的 callback 按操作类型分注册表，
// 只挂 Query 捕不到 DELETE 语句。
func captureSocialSQL(t *testing.T, fn func(*SocialRepository) error) []string {
	t.Helper()

	db := dryClientDB(t)
	var sqls []string
	capture := func(tx *gorm.DB) {
		// 带上 Vars：断言过滤值(如 auth_id 的实参)时要看真实绑定参数，
		// Statement.SQL 里只有 ? 占位符。
		sqls = append(sqls, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
	}
	if err := db.Callback().Query().After("gorm:query").
		Register("social_test:capture_query", capture); err != nil {
		t.Fatalf("注册查询回调失败: %v", err)
	}
	if err := db.Callback().Delete().After("gorm:delete").
		Register("social_test:capture_delete", capture); err != nil {
		t.Fatalf("注册删除回调失败: %v", err)
	}

	// DryRun 下不连库，返回的 error 无意义（Find 拿不到行、Delete 拿不到受影响行数），
	// 本文件只验 SQL 形状，故刻意忽略。
	_ = fn(NewSocialRepository(db))
	return sqls
}

// TestDeleteByIDIsPhysical 解绑必须是物理 DELETE，不能被改写成 UPDATE del_flag。
//
// 这条最值得钉：sys_social 的 DDL 里有 del_flag 列，哪天有人「顺手」给
// SysSocial 嵌上 repository.LogicDelete，删除就会静默变成软删——
// 而 sys_social 的查询路径不带 del_flag 条件，解绑后前端仍能看到那条绑定。
// 物理删除才是正确行为。
func TestDeleteByIDIsPhysical(t *testing.T) {
	stmts := captureSocialSQL(t, func(r *SocialRepository) error {
		_, err := r.DeleteByID(context.Background(), 42)
		return err
	})

	if len(stmts) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	sql := stmts[0]
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "DELETE") {
		t.Errorf("应为物理 DELETE，实际: %s", sql)
	}
	if strings.Contains(strings.ToLower(sql), "del_flag") {
		t.Errorf("不应出现 del_flag（那是逻辑删除的痕迹）: %s", sql)
	}
}

// TestSelectByAuthIDFiltersOnAuthID 按 auth_id 查重的 WHERE 形状。
// auth_id 是「这个三方账号」的全局唯一标识，查错列会让查重形同虚设。
func TestSelectByAuthIDFiltersOnAuthID(t *testing.T) {
	stmts := captureSocialSQL(t, func(r *SocialRepository) error {
		_, err := r.SelectByAuthID(context.Background(), "gitee123")
		return err
	})

	if len(stmts) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	if !strings.Contains(stmts[0], "auth_id") || !strings.Contains(stmts[0], "gitee123") {
		t.Errorf("WHERE 未按 auth_id 过滤: %s", stmts[0])
	}
}

// TestSelectByUserIDAndSourceFiltersBoth 续绑判定要同时按 user_id 与 source 定位，
// 只按 user_id 会把该用户在别的平台的绑定误判成「同平台已绑」。
func TestSelectByUserIDAndSourceFiltersBoth(t *testing.T) {
	stmts := captureSocialSQL(t, func(r *SocialRepository) error {
		_, err := r.SelectByUserIDAndSource(context.Background(), 7, "gitee")
		return err
	})

	if len(stmts) == 0 {
		t.Fatal("未捕获到 SQL")
	}
	for _, want := range []string{"user_id", "source"} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("WHERE 缺少 %s: %s", want, stmts[0])
		}
	}
}

// TestEmptyArgsSkipQuery 空参数直接返回空集，不该白跑一次库。
func TestEmptyArgsSkipQuery(t *testing.T) {
	if got := captureSocialSQL(t, func(r *SocialRepository) error {
		_, err := r.SelectByAuthID(context.Background(), "")
		return err
	}); len(got) != 0 {
		t.Errorf("空 authID 不应发 SQL, 实际: %v", got)
	}

	if got := captureSocialSQL(t, func(r *SocialRepository) error {
		_, err := r.SelectByUserIDAndSource(context.Background(), 0, "gitee")
		return err
	}); len(got) != 0 {
		t.Errorf("userID<=0 不应发 SQL, 实际: %v", got)
	}
}

// TestInvalidIDRejected 非法主键在进库前就被拒，避免拼出 WHERE id = 0 这类全表风险语句。
func TestInvalidIDRejected(t *testing.T) {
	r := NewSocialRepository(dryClientDB(t))
	if _, err := r.DeleteByID(context.Background(), 0); err == nil {
		t.Error("DeleteByID(0) 应报错")
	}
	if _, err := r.UpdateByID(context.Background(), 1, nil); err == nil {
		t.Error("UpdateByID 空更新列应报错")
	}
}
