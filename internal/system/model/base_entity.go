package model

import "time"

// BaseEntity 实体基类，对应 Java org.dromara.common.mybatis.core.domain.BaseEntity。
// 各实体匿名嵌入复用本审计字段；具体值由 service 层在落库前按登录态写入。
type BaseEntity struct {
	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`
}
