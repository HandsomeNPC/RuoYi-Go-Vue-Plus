package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

var ErrSocialNotFound = errors.New("repository: 社会化关系不存在")

// SocialRepository sys_social 数据访问。
type SocialRepository struct {
	db *gorm.DB
}

// NewSocialRepository 构造社会化关系 repository。
func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

// SelectByUserID 按用户ID查其绑定的社会化授权列表。
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

// SelectByID 按主键查一条社会化绑定，不存在时返回 ErrSocialNotFound。
func (r *SocialRepository) SelectByID(ctx context.Context, id int64) (*model.SysSocial, error) {
	if id <= 0 {
		return nil, ErrSocialNotFound
	}

	var row model.SysSocial
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSocialNotFound
		}
		return nil, fmt.Errorf("repository: 查询社会化绑定 id=%d 失败: %w", id, err)
	}
	return &row, nil
}

// SelectByAuthID 按 auth_id 查绑定关系。
//
// 返回 slice 而非单条：调用方按「非空即已被绑定」判定。
func (r *SocialRepository) SelectByAuthID(ctx context.Context,
	authID string) ([]*model.SysSocial, error) {

	if authID == "" {
		return nil, nil
	}

	var rows []*model.SysSocial
	if err := r.db.WithContext(ctx).
		Where("auth_id = ?", authID).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询 auth_id=%q 的社会化绑定失败: %w", authID, err)
	}
	return rows, nil
}

// SelectByUserIDAndSource 查某用户在某平台的绑定关系。
func (r *SocialRepository) SelectByUserIDAndSource(ctx context.Context, userID int64,
	source string) ([]*model.SysSocial, error) {

	if userID <= 0 || source == "" {
		return nil, nil
	}

	var rows []*model.SysSocial
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND source = ?", userID, source).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 在 %s 的绑定失败: %w", userID, source, err)
	}
	return rows, nil
}

// Insert 插入一条社会化绑定。
// id 无 auto_increment，主键须由 service 层预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *SocialRepository) Insert(ctx context.Context, e *model.SysSocial) error {
	if e == nil {
		return fmt.Errorf("repository: 社会化关系为空")
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return fmt.Errorf("repository: 插入社会化绑定失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「平台不再返回 refresh_token」
// 这类清空写不进库。update_by/update_time 由 pkg/repository 的更新回调补齐。
func (r *SocialRepository) UpdateByID(ctx context.Context, id int64,
	columns map[string]any) (int64, error) {

	if id <= 0 {
		return 0, fmt.Errorf("repository: 社会化关系主键无效: %d", id)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 社会化关系更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysSocial{}).
		Where("id = ?", id).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新社会化绑定 id=%d 失败: %w", id, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByID 按主键删除，返回受影响行数。
//
// 物理删除：SysSocial 未嵌 repository.LogicDelete
// （SysSocial 上没有 @TableLogic，只有 SysUser/SysRole/SysDept 等才有）。
func (r *SocialRepository) DeleteByID(ctx context.Context, id int64) (int64, error) {
	if id <= 0 {
		return 0, fmt.Errorf("repository: 社会化关系主键无效: %d", id)
	}

	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.SysSocial{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除社会化绑定 id=%d 失败: %w", id, res.Error)
	}
	return res.RowsAffected, nil
}
