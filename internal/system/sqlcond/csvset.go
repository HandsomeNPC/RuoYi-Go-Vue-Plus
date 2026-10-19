// Package sqlcond 构造跨数据库可移植的 SQL 片段。
//
// 这里只放"同一语义在各数据库方言下写法不同"的条件；普通查询条件直接写在
// repository 里即可，不必经过本包。
package sqlcond

import "strconv"

// CSVContainsInt 构造「逗号分隔串 column 中整段包含整数 v」的条件，
// 返回值可直接展开给 gorm 的 Where：
//
//	cond, args := sqlcond.CSVContainsInt("ancestors", deptID)
//	db.Where(cond, args...)
//
// 替代 MySQL 专有的 FIND_IN_SET(v, column) > 0——该函数在 PostgreSQL /
// SQL Server / Oracle 下均不存在，会直接报 42883 undefined_function。
// 这里只用 = 与 LIKE，是各方言都认的标准语法。
//
// 四个分支对应 v 在串中的四种位置：独占整串、串首、串尾、串中。
// 必须按整段比对而非子串匹配——LIKE '%100%' 会让 100 命中 "1001,1002"，
// 用在数据权限或消息可见性上就是越权。
//
// column 由调用方以字面量传入（非用户输入），故直接拼进 SQL;
// v 是 int64，十进制形态不含 LIKE 的 % 与 _，无需再做转义。
func CSVContainsInt(column string, v int64) (string, []any) {
	s := strconv.FormatInt(v, 10)
	cond := "(" + column + " = ?" +
		" OR " + column + " LIKE ?" +
		" OR " + column + " LIKE ?" +
		" OR " + column + " LIKE ?)"
	return cond, []any{s, s + ",%", "%," + s, "%," + s + ",%"}
}
