package repository

import (
	"gorm.io/gorm"

	"ruoyi-go-vue-plus/pkg/constant"
)

// 复用的 GORM Scope。
//
// 本层是唯一接触 GORM 的地方（见 CLAUDE.md 的分层约定），
// 通用查询条件收在这里，避免各 repository 各写一份。

// NotDeleted 只查未逻辑删除的记录，即 del_flag = '0'。
//
// # 为什么必须有这个 scope
//
// Java 侧靠 MyBatis-Plus 的 @TableLogic 注解，框架**自动**给每条 select
// 加上 `del_flag = '0'`、并把 delete 改写成 update。GORM 没有等价机制:
// 它的 gorm.DeletedAt 软删除走的是 `deleted_at IS NULL` 语义，
// 与原项目的 char(1) 标志位列对不上，硬套需要自定义类型且会改变列语义。
//
// 所以这件事只能手动做，而**漏一处就是一条能查出已删数据的路径** ——
// 那种 bug 不会报错，只会让被删的用户还能登录。抽成 scope 是为了让
// 「加这个条件」变成一个能被搜索、能被 review 的显式动作，
// 而不是散落在各处的 Where 字符串。
//
// 用法：db.Scopes(NotDeleted()).Where(...).First(&x)
func NotDeleted() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("del_flag = ?", constant.StatusNormal)
	}
}
