// Package repository GORM 数据访问公共设施。
package repository

import (
	"database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// del_flag 取值，对齐 RuoYi 的 char(1) 语义。
const (
	// FlagNotDeleted 未删除。
	FlagNotDeleted = "0"
	// FlagDeleted 已删除。
	FlagDeleted = "1"
)

// LogicDelete 供带 del_flag 列的实体嵌入，嵌入后该实体自动获得逻辑删除能力：
//   - 查询/更新自动追加 del_flag = '0'
//   - Delete 改写为 UPDATE ... SET del_flag = '1'
//   - Unscoped() 绕过以上全部，物理删除、查全量
//
// 未嵌入的实体不受任何影响，Delete 仍是物理删除。
type LogicDelete struct {
	// default:0 必须保留：没有它，插入零值时写进库的是空串而非 '0'，
	// 之后 del_flag = '0' 的查询条件将永远匹配不上该行。
	DelFlag DelFlag `gorm:"column:del_flag;default:0" json:"-"`
}

// DelFlag del_flag 字段类型。通过实现 GORM 的
// QueryClauses/UpdateClauses/DeleteClauses 三接口，在拼 SQL 前改写语句。
// 这些接口挂在字段类型上而非全局 callback，因此没有该字段的表天然不受影响。
type DelFlag string

// notDeleted 未删除的字段值，作为查询条件与软删除内置子句的基准值。
func notDeleted() sql.NullString {
	return sql.NullString{String: FlagNotDeleted, Valid: true}
}

// QueryClauses 查询自动追加 del_flag = '0'，复用 GORM 内置软删除查询子句。
func (DelFlag) QueryClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{gorm.SoftDeleteQueryClause{Field: f, ZeroValue: notDeleted()}}
}

// UpdateClauses 更新自动追加 del_flag = '0'，避免改到已删除的行。
func (DelFlag) UpdateClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{gorm.SoftDeleteUpdateClause{Field: f, ZeroValue: notDeleted()}}
}

// DeleteClauses 把 DELETE 改写成 UPDATE ... SET del_flag = '1'。
func (DelFlag) DeleteClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{logicDeleteClause{Field: f}}
}

// logicDeleteClause 逻辑删除子句，仿 gorm.SoftDeleteDeleteClause 实现，
// 差别只在 SET 的值是常量 '1' 而不是当前时间。
type logicDeleteClause struct {
	Field *schema.Field
}

func (logicDeleteClause) Name() string               { return "" }
func (logicDeleteClause) Build(clause.Builder)       {}
func (logicDeleteClause) MergeClause(*clause.Clause) {}

// ModifyStatement 在删除语句构建前介入改写；Unscoped 时不介入，退化为物理删除。
func (c logicDeleteClause) ModifyStatement(stmt *gorm.Statement) {
	if stmt.SQL.Len() > 0 || stmt.Unscoped {
		return
	}

	stmt.AddClause(clause.Set{{
		Column: clause.Column{Name: c.Field.DBName},
		Value:  FlagDeleted,
	}})
	stmt.SetColumn(c.Field.DBName, FlagDeleted, true)

	// Delete(&SysUser{UserID: 1}) 这类按实体主键删除，把主键值补成 WHERE 条件。
	if stmt.Schema != nil {
		_, queryValues := schema.GetIdentityFieldValuesMap(
			stmt.Context, stmt.ReflectValue, stmt.Schema.PrimaryFields)
		column, values := schema.ToQueryValues(
			stmt.Table, stmt.Schema.PrimaryFieldDBNames, queryValues)
		if len(values) > 0 {
			stmt.AddClause(clause.Where{Exprs: []clause.Expression{
				clause.IN{Column: column, Values: values},
			}})
		}
	}

	// 复用查询子句追加 del_flag = '0'，避免重复删除已删除的行。
	gorm.SoftDeleteQueryClause{Field: c.Field, ZeroValue: notDeleted()}.ModifyStatement(stmt)
	stmt.AddClauseIfNotExists(clause.Update{})
	stmt.Build(stmt.DB.Callback().Update().Clauses...)
}
