package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// SocialRepository sys_social 数据访问。
type SocialRepository struct {
	db *gorm.DB
}

// NewSocialRepository 构造社会化关系 repository。
func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

// SelectByUserID 按用户ID查其绑定的社会化授权列表（对应 Java SysSocialMapper#queryListByUserId）。
// sys_social 未映射 del_flag，这里直接物理查询。
func (r *SocialRepository) SelectByUserID(ctx context.Context,
	userID int64) ([]*model.SysSocial, error) {

	if userID <= 0 {
		return nil, nil
	}

	var rows []*model.SysSocial
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 社会化绑定失败: %w", userID, err)
	}
	return rows, nil
}
