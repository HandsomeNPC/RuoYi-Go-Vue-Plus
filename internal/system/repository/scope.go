package repository

import (
	"gorm.io/gorm"

	"ruoyi-go-vue-plus/pkg/constant"
)

// NotDeleted 只查未逻辑删除的记录，即 del_flag = '0'。
func NotDeleted() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("del_flag = ?", constant.StatusNormal)
	}
}
