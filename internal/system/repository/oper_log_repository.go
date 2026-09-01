package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// OperLogRepository sys_oper_log 数据访问。
type OperLogRepository struct {
	db *gorm.DB
}

// NewOperLogRepository 构造操作日志 repository。
func NewOperLogRepository(db *gorm.DB) *OperLogRepository {
	return &OperLogRepository{db: db}
}

// Insert 插入一条操作日志。
// oper_id 无 auto_increment，主键须由调用方（service 层）预先填好。
func (r *OperLogRepository) Insert(ctx context.Context, l *model.SysOperLog) error {
	if l == nil {
		return fmt.Errorf("repository: 操作日志为空")
	}
	if err := r.db.WithContext(ctx).Create(l).Error; err != nil {
		return fmt.Errorf("repository: 插入操作日志失败: %w", err)
	}
	return nil
}
