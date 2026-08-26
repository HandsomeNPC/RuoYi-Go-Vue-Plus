package repository

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// flagUser 带 del_flag 的实体。
type flagUser struct {
	UserID   int64  `gorm:"column:user_id;primaryKey"`
	UserName string `gorm:"column:user_name"`
	Status   string `gorm:"column:status"`
	LogicDelete
}

func (flagUser) TableName() string { return "flag_user" }

// plainRel 不带 del_flag 的实体。
type plainRel struct {
	UserID int64 `gorm:"column:user_id;primaryKey"`
	RoleID int64 `gorm:"column:role_id;primaryKey"`
}

func (plainRel) TableName() string { return "plain_rel" }

// dryDB DryRun 模式的 gorm 实例，只拼 SQL 不连库。
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

// sqlOf 取语句最终 SQL，构建出错直接失败。
func sqlOf(t *testing.T, tx *gorm.DB) string {
	t.Helper()
	if tx.Error != nil {
		t.Fatalf("构建 SQL 失败: %v", tx.Error)
	}
	return tx.Statement.SQL.String()
}

// TestQueryAppendsDelFlag 验证查询自动追加 del_flag 条件。
func TestQueryAppendsDelFlag(t *testing.T) {
	db := dryDB(t)

	var u flagUser
	got := sqlOf(t, db.Where("user_name = ?", "admin").First(&u).Statement.DB)
	want := "SELECT * FROM `flag_user` WHERE user_name = ? AND `flag_user`.`del_flag` = ? ORDER BY `flag_user`.`user_id` LIMIT ?"
	if got != want {
		t.Errorf("First SQL =\n%s\nwant\n%s", got, want)
	}

	var us []flagUser
	got = sqlOf(t, db.Where("status = ?", "0").Find(&us).Statement.DB)
	want = "SELECT * FROM `flag_user` WHERE status = ? AND `flag_user`.`del_flag` = ?"
	if got != want {
		t.Errorf("Find SQL =\n%s\nwant\n%s", got, want)
	}
}

// TestQueryOrConditionWrapped 验证 OR 条件被括号包裹后再 AND del_flag。
func TestQueryOrConditionWrapped(t *testing.T) {
	var us []flagUser
	got := sqlOf(t, dryDB(t).
		Where("user_name = ?", "a").Or("user_name = ?", "b").
		Find(&us).Statement.DB)
	want := "SELECT * FROM `flag_user` WHERE (user_name = ? OR user_name = ?) AND `flag_user`.`del_flag` = ?"
	if got != want {
		t.Errorf("OR SQL =\n%s\nwant\n%s", got, want)
	}
}

// TestCreateFillsDefaultFlag 验证零值插入时 del_flag 落 '0' 而非空串。
func TestCreateFillsDefaultFlag(t *testing.T) {
	tx := dryDB(t).Create(&flagUser{UserID: 1, UserName: "a"})
	got := sqlOf(t, tx)
	if !strings.Contains(got, "`del_flag`") {
		t.Fatalf("INSERT 应包含 del_flag 列: %s", got)
	}
	found := false
	for _, v := range tx.Statement.Vars {
		if v == FlagNotDeleted {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("INSERT 参数应含 %q, got %v", FlagNotDeleted, tx.Statement.Vars)
	}
}

// TestUpdateAppendsDelFlag 验证更新自动追加 del_flag 条件。
func TestUpdateAppendsDelFlag(t *testing.T) {
	got := sqlOf(t, dryDB(t).Model(&flagUser{}).
		Where("user_id = ?", 1).
		Updates(map[string]any{"status": "1"}).Statement.DB)
	want := "UPDATE `flag_user` SET `status`=? WHERE user_id = ? AND `flag_user`.`del_flag` = ?"
	if got != want {
		t.Errorf("UPDATE SQL =\n%s\nwant\n%s", got, want)
	}
}

// TestDeleteBecomesLogicUpdate 验证删除改写为 UPDATE SET del_flag = '1'。
func TestDeleteBecomesLogicUpdate(t *testing.T) {
	db := dryDB(t)

	got := sqlOf(t, db.Delete(&flagUser{UserID: 1}).Statement.DB)
	want := "UPDATE `flag_user` SET `del_flag`=? WHERE `flag_user`.`user_id` = ? AND `flag_user`.`del_flag` = ?"
	if got != want {
		t.Errorf("按主键删除 SQL =\n%s\nwant\n%s", got, want)
	}

	got = sqlOf(t, db.Where("user_id IN ?", []int64{1, 2}).Delete(&flagUser{}).Statement.DB)
	want = "UPDATE `flag_user` SET `del_flag`=? WHERE user_id IN (?,?) AND `flag_user`.`del_flag` = ?"
	if got != want {
		t.Errorf("按条件删除 SQL =\n%s\nwant\n%s", got, want)
	}
}

// TestDeleteSetsFlagDeleted 验证 SET 的值是 '1'。
func TestDeleteSetsFlagDeleted(t *testing.T) {
	tx := dryDB(t).Delete(&flagUser{UserID: 1})
	if tx.Error != nil {
		t.Fatalf("构建 SQL 失败: %v", tx.Error)
	}
	if len(tx.Statement.Vars) == 0 || tx.Statement.Vars[0] != FlagDeleted {
		t.Errorf("首个参数应为 %q, got %v", FlagDeleted, tx.Statement.Vars)
	}
}

// TestUnscopedBypasses 验证 Unscoped 物理删除、查全量。
func TestUnscopedBypasses(t *testing.T) {
	db := dryDB(t)

	got := sqlOf(t, db.Unscoped().Delete(&flagUser{UserID: 1}).Statement.DB)
	want := "DELETE FROM `flag_user` WHERE `flag_user`.`user_id` = ?"
	if got != want {
		t.Errorf("Unscoped 删除 SQL =\n%s\nwant\n%s", got, want)
	}

	var us []flagUser
	got = sqlOf(t, db.Unscoped().Find(&us).Statement.DB)
	want = "SELECT * FROM `flag_user`"
	if got != want {
		t.Errorf("Unscoped 查询 SQL =\n%s\nwant\n%s", got, want)
	}
}

// TestPlainTableUnaffected 验证无 del_flag 的表不受任何影响。
func TestPlainTableUnaffected(t *testing.T) {
	db := dryDB(t)

	var rs []plainRel
	got := sqlOf(t, db.Find(&rs).Statement.DB)
	want := "SELECT * FROM `plain_rel`"
	if got != want {
		t.Errorf("查询 SQL =\n%s\nwant\n%s", got, want)
	}

	got = sqlOf(t, db.Where("user_id = ?", 1).Delete(&plainRel{}).Statement.DB)
	want = "DELETE FROM `plain_rel` WHERE user_id = ?"
	if got != want {
		t.Errorf("删除 SQL =\n%s\nwant\n%s", got, want)
	}
}
