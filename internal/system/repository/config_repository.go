package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrConfigNotFound 参数配置不存在。
var ErrConfigNotFound = errors.New("repository: 参数配置不存在")

// ConfigRepository sys_config 数据访问。
type ConfigRepository struct {
	db *gorm.DB
}

// NewConfigRepository 构造参数配置 repository。
func NewConfigRepository(db *gorm.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// SelectByID 按主键查参数配置，不存在时返回 ErrConfigNotFound。
func (r *ConfigRepository) SelectByID(ctx context.Context, configID int64) (*model.SysConfig, error) {
	if configID <= 0 {
		return nil, ErrConfigNotFound
	}

	var cfg model.SysConfig
	err := r.db.WithContext(ctx).
		Where("config_id = ?", configID).
		First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("repository: 查询参数配置 id=%d 失败: %w", configID, err)
	}
	return &cfg, nil
}

// SelectByKey 按参数键名查参数配置，不存在时返回 ErrConfigNotFound。
func (r *ConfigRepository) SelectByKey(ctx context.Context, configKey string) (*model.SysConfig, error) {
	if configKey == "" {
		return nil, ErrConfigNotFound
	}

	var cfg model.SysConfig
	err := r.db.WithContext(ctx).
		Where("config_key = ?", configKey).
		First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("repository: 查询参数配置 %q 失败: %w", configKey, err)
	}
	return &cfg, nil
}

// SelectByIDs 按主键批量查参数配置，返回实际命中的行（缺失主键静默跳过，由调用方比对数量）。
func (r *ConfigRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysConfig, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysConfig
	if err := r.db.WithContext(ctx).Where("config_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询参数配置 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查参数配置。
func (r *ConfigRepository) SelectPageList(ctx context.Context, q bo.SysConfigQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysConfig], error) {

	db := applyConfigQuery(r.db.WithContext(ctx).Model(&model.SysConfig{}), q)
	// 仅在调用方未指定排序时按主键升序兜底（对齐 Java orderByAsc(SysConfig::getConfigId)）。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("config_id")
	}

	var rows []*model.SysConfig
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查参数配置，供导出等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyConfigQuery，保证两种路径的过滤条件永不漂移。
func (r *ConfigRepository) SelectList(ctx context.Context, q bo.SysConfigQueryBo,
	limit int) ([]*model.SysConfig, error) {

	db := applyConfigQuery(r.db.WithContext(ctx).Model(&model.SysConfig{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定按主键升序，保证输出顺序稳定。
	db = db.Order("config_id")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysConfig
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询参数配置列表失败: %w", err)
	}
	return rows, nil
}

// applyConfigQuery 应用参数配置查询条件（对齐 Java buildQueryWrapper）：
// 名称/键名走 like，类型走 eq，创建时间走闭区间；空串一概不筛。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyConfigQuery(db *gorm.DB, q bo.SysConfigQueryBo) *gorm.DB {
	if q.ConfigName != "" {
		db = db.Where("config_name LIKE ?", "%"+escapeLike(q.ConfigName)+"%")
	}
	if q.ConfigKey != "" {
		db = db.Where("config_key LIKE ?", "%"+escapeLike(q.ConfigKey)+"%")
	}
	if q.ConfigType != "" {
		db = db.Where("config_type = ?", q.ConfigType)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	return db
}

// likeEscaper 转义 LIKE 模式里的元字符，使入参按字面量匹配。
// 不转义的话搜 "%" 会命中全表、"_" 会变成任意单字符通配——
// 这是与 Java likeIfText（不转义）的有意差异，那边搜含 % 的参数名同样会跑偏。
// 反斜杠必须排在最前：否则会把后两条刚补上的转义符再转一次。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLike 见 likeEscaper。
func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// Insert 插入一条参数配置。
// config_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *ConfigRepository) Insert(ctx context.Context, cfg *model.SysConfig) error {
	if cfg == nil {
		return fmt.Errorf("repository: 参数配置为空")
	}
	if err := r.db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("repository: 插入参数配置失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *ConfigRepository) UpdateByID(ctx context.Context, configID int64,
	columns map[string]any) (int64, error) {

	if configID <= 0 {
		return 0, fmt.Errorf("repository: 参数配置主键无效: %d", configID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 参数配置更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysConfig{}).
		Where("config_id = ?", configID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新参数配置 id=%d 失败: %w", configID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateByKey 按参数键名更新指定列，返回受影响行数（0 表示键名不存在）。
// 对应 Java updateConfig 中 configId 为 null 时走的 eq(configKey).updateCount 分支。
func (r *ConfigRepository) UpdateByKey(ctx context.Context, configKey string,
	columns map[string]any) (int64, error) {

	if configKey == "" {
		return 0, fmt.Errorf("repository: 参数键名为空")
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 参数配置更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysConfig{}).
		Where("config_key = ?", configKey).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新参数配置 %q 失败: %w", configKey, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_config 无 del_flag，这是物理删除（对齐 Java 侧该表未开逻辑删除）。
func (r *ConfigRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 参数配置主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("config_id IN ?", ids).
		Delete(&model.SysConfig{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除参数配置 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByConfigKey 判断 config_key 是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java neIfPresent，供修改场景排除自身）。
func (r *ConfigRepository) ExistsByConfigKey(ctx context.Context, configKey string,
	excludeID int64) (bool, error) {

	if configKey == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysConfig{}).Where("config_key = ?", configKey)
	if excludeID > 0 {
		db = db.Where("config_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验参数键名 %q 失败: %w", configKey, err)
	}
	return count > 0, nil
}
