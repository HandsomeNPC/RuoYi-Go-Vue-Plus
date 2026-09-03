package model

import "time"

// BaseEntity 审计字段，嵌入各实体。
//
// 与 internal/system/model 的同名类型形状一致而非共用：resource 与 system 是独立模块，
// 跨模块 import model 内部会破坏模块隔离。pkg/repository 的审计回调按列名
// （create_by/create_dept/create_time/update_by/update_time）识别，不认类型，
// 各模块各持一份不影响回调生效。
type BaseEntity struct {
	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`
}
