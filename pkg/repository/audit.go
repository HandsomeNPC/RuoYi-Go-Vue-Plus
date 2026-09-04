package repository

import (
	"context"
	"reflect"
	"time"

	"gorm.io/gorm"
)

// AuditDefaultUserID 取不到登录用户时写入的兜底值。
const AuditDefaultUserID int64 = -1

// 审计列名。回调按列名而非 Go 类型识别字段，因此 pkg/repository 无需 import
// 实体所在的 internal 包（那会成环），且不含这些列的表天然不受影响。
const (
	colCreateDept = "create_dept"
	colCreateBy   = "create_by"
	colCreateTime = "create_time"
	colUpdateBy   = "update_by"
	colUpdateTime = "update_time"
)

// AuditUser 审计字段的来源身份。
type AuditUser struct {
	UserID int64
	DeptID int64
}

type auditUserCtxKey struct{}

// WithAuditUser 把操作者身份放进 ctx，供审计回调取用。
// 由 HTTP 层的中间件调用——回调只拿得到 *gorm.DB，登录态只能靠 ctx 接力。
func WithAuditUser(ctx context.Context, u AuditUser) context.Context {
	return context.WithValue(ctx, auditUserCtxKey{}, u)
}

// AuditUserFrom 取出 ctx 中的操作者身份。
func AuditUserFrom(ctx context.Context) (AuditUser, bool) {
	if ctx == nil {
		return AuditUser{}, false
	}
	u, ok := ctx.Value(auditUserCtxKey{}).(AuditUser)
	return u, ok
}

// RegisterAuditCallbacks 注册审计字段自动填充回调。
//
// Before("gorm:create") 落在用户钩子之后、ConvertToCreateValues 读字段之前；
// Before("gorm:update") 落在 gorm:setup_reflect_value 之后，故 ReflectValue 已就绪。
func RegisterAuditCallbacks(db *gorm.DB) error {
	if err := db.Callback().Create().Before("gorm:create").
		Register("ruoyi:audit_insert_fill", auditInsertFill); err != nil {
		return err
	}
	return db.Callback().Update().Before("gorm:update").
		Register("ruoyi:audit_update_fill", auditUpdateFill)
}

// resolveAuditUser 解析操作者，缺登录态时回落 -1。
func resolveAuditUser(ctx context.Context) AuditUser {
	u, ok := AuditUserFrom(ctx)
	if !ok || u.UserID == 0 {
		return AuditUser{UserID: AuditDefaultUserID, DeptID: AuditDefaultUserID}
	}
	return u
}

// auditInsertFill 插入前填充创建/更新审计字段。
//
// 逐行反射赋值而非走 stmt.SetColumn：后者在回调期会拿同一个值遍历整个 slice，
// 批量插入时会把各行预设的 create_time 冲掉。
func auditInsertFill(db *gorm.DB) {
	stmt := db.Statement
	if db.Error != nil || stmt == nil || stmt.Schema == nil {
		return
	}

	now := time.Now()
	user := resolveAuditUser(stmt.Context)

	eachRow(stmt.ReflectValue, func(row reflect.Value) {
		// create_time 已有值时以它为基准，让 update_time 与之一致。
		current := now
		if f := stmt.Schema.FieldsByDBName[colCreateTime]; f != nil {
			if v, zero := f.ValueOf(stmt.Context, row); !zero {
				if t, ok := asTime(v); ok {
					current = t
				}
			}
			_ = f.Set(stmt.Context, row, current)
		}
		if f := stmt.Schema.FieldsByDBName[colUpdateTime]; f != nil {
			_ = f.Set(stmt.Context, row, current)
		}

		// 已指定创建人则三个人员字段整体不动。
		createBy := stmt.Schema.FieldsByDBName[colCreateBy]
		if createBy == nil {
			return
		}
		if _, zero := createBy.ValueOf(stmt.Context, row); !zero {
			return
		}
		_ = createBy.Set(stmt.Context, row, user.UserID)
		if f := stmt.Schema.FieldsByDBName[colUpdateBy]; f != nil {
			_ = f.Set(stmt.Context, row, user.UserID)
		}
		if f := stmt.Schema.FieldsByDBName[colCreateDept]; f != nil {
			if _, zero := f.ValueOf(stmt.Context, row); zero {
				_ = f.Set(stmt.Context, row, user.DeptID)
			}
		}
	})
}

// auditUpdateFill 更新前无条件刷新 update_by/update_time。
//
// 这里必须用 SetColumn：Updates(map) / Update(col, val) 的 Dest 是 map，逐行反射覆盖不到。
func auditUpdateFill(db *gorm.DB) {
	stmt := db.Statement
	if db.Error != nil || stmt == nil || stmt.Schema == nil {
		return
	}

	if stmt.Schema.FieldsByDBName[colUpdateTime] != nil {
		stmt.SetColumn(colUpdateTime, time.Now(), true)
	}
	if stmt.Schema.FieldsByDBName[colUpdateBy] != nil {
		stmt.SetColumn(colUpdateBy, resolveAuditUser(stmt.Context).UserID, true)
	}
}

// eachRow 对 Struct 目标回调一次，对 Slice/Array 目标逐元素回调。
func eachRow(rv reflect.Value, fn func(reflect.Value)) {
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			row := reflect.Indirect(rv.Index(i))
			if row.Kind() == reflect.Struct {
				fn(row)
			}
		}
	case reflect.Struct:
		if rv.CanAddr() {
			fn(rv)
		}
	}
}

// asTime 从字段值取出 time.Time，兼容 *time.Time 字段。
func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t != nil {
			return *t, true
		}
	}
	return time.Time{}, false
}
