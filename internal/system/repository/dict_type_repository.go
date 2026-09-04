package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

var ErrDictTypeNotFound = errors.New("repository: 字典类型不存在")

// DictTypeRepository sys_dict_type 数据访问。
type DictTypeRepository struct {
	db *gorm.DB
}

// NewDictTypeRepository 构造字典类型 repository。
func NewDictTypeRepository(db *gorm.DB) *DictTypeRepository {
	return &DictTypeRepository{db: db}
}

// SelectByID 按主键查字典类型，不存在时返回 ErrDictTypeNotFound。
func (r *DictTypeRepository) SelectByID(ctx context.Context, dictID int64) (*model.SysDictType, error) {
	if dictID <= 0 {
		return nil, ErrDictTypeNotFound
	}

	var dict model.SysDictType
	err := r.db.WithContext(ctx).
		Where("dict_id = ?", dictID).
		First(&dict).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDictTypeNotFound
		}
		return nil, fmt.Errorf("repository: 查询字典类型 id=%d 失败: %w", dictID, err)
	}
	return &dict, nil
}

// SelectByType 按字典类型查字典类型，不存在时返回 ErrDictTypeNotFound。
// dict_type 有唯一索引，故 First 取到的必是唯一一条。
func (r *DictTypeRepository) SelectByType(ctx context.Context, dictType string) (*model.SysDictType, error) {
	if dictType == "" {
		return nil, ErrDictTypeNotFound
	}

	var dict model.SysDictType
	err := r.db.WithContext(ctx).
		Where("dict_type = ?", dictType).
		First(&dict).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDictTypeNotFound
		}
		return nil, fmt.Errorf("repository: 查询字典类型 %q 失败: %w", dictType, err)
	}
	return &dict, nil
}

// SelectByIDs 按主键批量查，返回实际命中的行（缺失主键静默跳过，由调用方比对数量）。
func (r *DictTypeRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysDictType, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysDictType
	if err := r.db.WithContext(ctx).Where("dict_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询字典类型 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查字典类型。
func (r *DictTypeRepository) SelectPageList(ctx context.Context, q bo.SysDictTypeQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysDictType], error) {

	db := applyDictTypeQuery(r.db.WithContext(ctx).Model(&model.SysDictType{}), q)
	// 仅在调用方未指定排序时按主键升序兜底。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("dict_id")
	}

	var rows []*model.SysDictType
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查字典类型，供导出与下拉选择等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyDictTypeQuery，保证两种路径的过滤条件永不漂移。
func (r *DictTypeRepository) SelectList(ctx context.Context, q bo.SysDictTypeQueryBo,
	limit int) ([]*model.SysDictType, error) {

	db := applyDictTypeQuery(r.db.WithContext(ctx).Model(&model.SysDictType{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定按主键升序，保证输出顺序稳定。
	db = db.Order("dict_id")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysDictType
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询字典类型列表失败: %w", err)
	}
	return rows, nil
}

// applyDictTypeQuery 应用字典类型查询条件：
// 名称与类型均走 LIKE，空串一概不筛。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyDictTypeQuery(db *gorm.DB, q bo.SysDictTypeQueryBo) *gorm.DB {
	if q.DictName != "" {
		db = db.Where("dict_name LIKE ?", "%"+escapeLike(q.DictName)+"%")
	}
	if q.DictType != "" {
		db = db.Where("dict_type LIKE ?", "%"+escapeLike(q.DictType)+"%")
	}
	return db
}

// Insert 插入一条字典类型。
// dict_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *DictTypeRepository) Insert(ctx context.Context, dict *model.SysDictType) error {
	if dict == nil {
		return fmt.Errorf("repository: 字典类型为空")
	}
	if err := r.db.WithContext(ctx).Create(dict).Error; err != nil {
		return fmt.Errorf("repository: 插入字典类型失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *DictTypeRepository) UpdateByID(ctx context.Context, dictID int64,
	columns map[string]any) (int64, error) {

	if dictID <= 0 {
		return 0, fmt.Errorf("repository: 字典类型主键无效: %d", dictID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 字典类型更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysDictType{}).
		Where("dict_id = ?", dictID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新字典类型 id=%d 失败: %w", dictID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_dict_type 无 del_flag，这是物理删除。
func (r *DictTypeRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 字典类型主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("dict_id IN ?", ids).
		Delete(&model.SysDictType{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除字典类型 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByDictType 判断 dict_type 是否已被占用，excludeID > 0 时排除该主键
// （供修改场景排除自身）。
func (r *DictTypeRepository) ExistsByDictType(ctx context.Context, dictType string,
	excludeID int64) (bool, error) {

	if dictType == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysDictType{}).Where("dict_type = ?", dictType)
	if excludeID > 0 {
		db = db.Where("dict_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验字典类型 %q 失败: %w", dictType, err)
	}
	return count > 0, nil
}
