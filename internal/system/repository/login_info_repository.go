package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// LoginInfoRepository sys_login_info 数据访问。
type LoginInfoRepository struct {
	db *gorm.DB
}

// NewLoginInfoRepository 构造登录日志 repository。
func NewLoginInfoRepository(db *gorm.DB) *LoginInfoRepository {
	return &LoginInfoRepository{db: db}
}

// Insert 插入一条登录日志。
// info_id 无 auto_increment，主键须由调用方（service 层）预先填好。
func (r *LoginInfoRepository) Insert(ctx context.Context, info *model.SysLoginInfo) error {
	if info == nil {
		return fmt.Errorf("repository: 登录日志为空")
	}
	if err := r.db.WithContext(ctx).Create(info).Error; err != nil {
		return fmt.Errorf("repository: 插入登录日志失败: %w", err)
	}
	return nil
}
