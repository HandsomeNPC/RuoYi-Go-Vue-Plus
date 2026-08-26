package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// SelectPage 执行分页查询：先 COUNT 总数，再按分页与排序取当页数据。
//
// db 须由调用方拼好 Model/Table、WHERE、JOIN；本函数只补 COUNT、ORDER BY、LIMIT/OFFSET。
// dest 常是 VO 而非实体，故不能拿 dest 反推表名——会打到不存在的表并丢掉逻辑删除条件。
func SelectPage[T any](db *gorm.DB, q PageQuery, dest *[]T) (PageResult[T], error) {
	order, err := q.OrderBy()
	if err != nil {
		return EmptyPage[T](), err
	}

	// 新建 session，使 Count 与 Find 各自克隆 statement，避免 SELECT count(*) 污染 Find。
	session := db.Session(&gorm.Session{})
	if db.Statement.Model == nil && db.Statement.Table == "" {
		session = session.Model(dest)
	}

	var total int64
	if err := session.Count(&total).Error; err != nil {
		return EmptyPage[T](), fmt.Errorf("repository: 分页统计总数失败: %w", err)
	}
	if total == 0 {
		return EmptyPage[T](), nil
	}

	tx := session.Scopes(q.Paginate())
	if len(order.Columns) > 0 {
		tx = tx.Order(order)
	}
	if err := tx.Find(dest).Error; err != nil {
		return EmptyPage[T](), fmt.Errorf("repository: 分页查询数据失败: %w", err)
	}

	return Page(*dest, total), nil
}

// SelectPageCtx 语义同 SelectPage，额外绑定 ctx。
func SelectPageCtx[T any](ctx context.Context, db *gorm.DB, q PageQuery, dest *[]T) (PageResult[T], error) {
	return SelectPage(db.WithContext(ctx), q, dest)
}
