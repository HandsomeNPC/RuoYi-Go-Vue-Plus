package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/resource/model"
	"ruoyi-go-vue-plus/internal/resource/model/bo"
	"ruoyi-go-vue-plus/pkg/constant"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrOssConfigNotFound 对象存储配置不存在。
var ErrOssConfigNotFound = errors.New("repository: 对象存储配置不存在")

// OssConfigRepository sys_oss_config 数据访问。
type OssConfigRepository struct {
	db *gorm.DB
}

// NewOssConfigRepository 构造对象存储配置 repository。
func NewOssConfigRepository(db *gorm.DB) *OssConfigRepository {
	return &OssConfigRepository{db: db}
}

// SelectByID 按主键查配置，不存在时返回 ErrOssConfigNotFound。
func (r *OssConfigRepository) SelectByID(ctx context.Context,
	ossConfigID int64) (*model.SysOssConfig, error) {

	if ossConfigID <= 0 {
		return nil, ErrOssConfigNotFound
	}

	var cfg model.SysOssConfig
	err := r.db.WithContext(ctx).Where("oss_config_id = ?", ossConfigID).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOssConfigNotFound
		}
		return nil, fmt.Errorf("repository: 查询对象存储配置 id=%d 失败: %w", ossConfigID, err)
	}
	return &cfg, nil
}

// SelectByIDs 按主键批量查，返回实际命中的行。
func (r *OssConfigRepository) SelectByIDs(ctx context.Context,
	ids []int64) ([]*model.SysOssConfig, error) {

	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysOssConfig
	if err := r.db.WithContext(ctx).Where("oss_config_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询对象存储配置 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectAll 查全部配置，供启动时预热缓存用。
// 配置表行数极少（seed 只有 5 行），不分页。
func (r *OssConfigRepository) SelectAll(ctx context.Context) ([]*model.SysOssConfig, error) {
	var rows []*model.SysOssConfig
	err := r.db.WithContext(ctx).Model(&model.SysOssConfig{}).
		Order("oss_config_id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询对象存储配置列表失败: %w", err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查配置。
func (r *OssConfigRepository) SelectPageList(ctx context.Context, q bo.SysOssConfigQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysOssConfig], error) {

	db := applyOssConfigQuery(r.db.WithContext(ctx).Model(&model.SysOssConfig{}), q)
	if !page.HasOrder() {
		db = db.Order("oss_config_id")
	}

	var rows []*model.SysOssConfig
	return pkgrepo.SelectPage(db, page, &rows)
}

// applyOssConfigQuery 应用配置查询条件（对齐 Java buildQueryWrapper）：
// 配置键与状态走 eq，桶名走 like。
func applyOssConfigQuery(db *gorm.DB, q bo.SysOssConfigQueryBo) *gorm.DB {
	if q.ConfigKey != "" {
		db = db.Where("config_key = ?", q.ConfigKey)
	}
	if q.BucketName != "" {
		db = db.Where("bucket_name LIKE ?", "%"+escapeLike(q.BucketName)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	return db
}

// Insert 插入一条配置。
// oss_config_id 无 auto_increment，主键须由 service 层预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *OssConfigRepository) Insert(ctx context.Context, cfg *model.SysOssConfig) error {
	if cfg == nil {
		return fmt.Errorf("repository: 对象存储配置为空")
	}
	if err := r.db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("repository: 插入对象存储配置失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在或值无变化）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空前缀/备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *OssConfigRepository) UpdateByID(ctx context.Context, ossConfigID int64,
	columns map[string]any) (int64, error) {

	if ossConfigID <= 0 {
		return 0, fmt.Errorf("repository: 对象存储配置主键无效: %d", ossConfigID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 对象存储配置更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysOssConfig{}).
		Where("oss_config_id = ?", ossConfigID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新对象存储配置 id=%d 失败: %w", ossConfigID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateStatus 切换默认配置：先把全表刷成非默认，再把目标行置为传入状态。
//
// 两条 update 必须同事务（对齐 Java 的 @Transactional）：中途失败会留下
// 「一个默认都没有」的状态，此后所有上传都拿不到默认配置而失败。
// 全表 update 无 WHERE 是有意的——默认配置全局唯一，逐个查出来再改反而更慢。
func (r *OssConfigRepository) UpdateStatus(ctx context.Context, ossConfigID int64,
	status string) error {

	if ossConfigID <= 0 {
		return fmt.Errorf("repository: 对象存储配置主键无效: %d", ossConfigID)
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// gorm 默认拦截无条件全表更新，这里是刻意为之，用 Where("1 = 1") 放行。
		if err := tx.Model(&model.SysOssConfig{}).Where("1 = 1").
			Update("status", constant.No).Error; err != nil {
			return fmt.Errorf("repository: 重置对象存储配置默认标记失败: %w", err)
		}
		if err := tx.Model(&model.SysOssConfig{}).
			Where("oss_config_id = ?", ossConfigID).
			Update("status", status).Error; err != nil {
			return fmt.Errorf("repository: 更新对象存储配置 id=%d 状态失败: %w", ossConfigID, err)
		}
		return nil
	})
	return err
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_oss_config 无 del_flag，物理删除。
func (r *OssConfigRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 对象存储配置主键为空")
	}

	res := r.db.WithContext(ctx).Where("oss_config_id IN ?", ids).Delete(&model.SysOssConfig{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除对象存储配置 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByConfigKey 判断 config_key 是否已被占用，excludeID > 0 时排除该主键
// （供修改场景排除自身，改回原键不算冲突）。
func (r *OssConfigRepository) ExistsByConfigKey(ctx context.Context, configKey string,
	excludeID int64) (bool, error) {

	if configKey == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysOssConfig{}).Where("config_key = ?", configKey)
	if excludeID > 0 {
		db = db.Where("oss_config_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验配置键 %q 失败: %w", configKey, err)
	}
	return count > 0, nil
}
