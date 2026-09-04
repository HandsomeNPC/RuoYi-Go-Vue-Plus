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

var ErrDictDataNotFound = errors.New("repository: 字典数据不存在")

// DictDataRepository sys_dict_data 数据访问。
type DictDataRepository struct {
	db *gorm.DB
}

// NewDictDataRepository 构造字典数据 repository。
func NewDictDataRepository(db *gorm.DB) *DictDataRepository {
	return &DictDataRepository{db: db}
}

// SelectByID 按字典编码查字典数据，不存在时返回 ErrDictDataNotFound。
func (r *DictDataRepository) SelectByID(ctx context.Context, dictCode int64) (*model.SysDictData, error) {
	if dictCode <= 0 {
		return nil, ErrDictDataNotFound
	}

	var data model.SysDictData
	err := r.db.WithContext(ctx).
		Where("dict_code = ?", dictCode).
		First(&data).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDictDataNotFound
		}
		return nil, fmt.Errorf("repository: 查询字典数据 code=%d 失败: %w", dictCode, err)
	}
	return &data, nil
}

// SelectByIDs 按字典编码批量查，返回实际命中的行（缺失主键静默跳过，由调用方比对数量）。
func (r *DictDataRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysDictData, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysDictData
	if err := r.db.WithContext(ctx).Where("dict_code IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询字典数据 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectByType 按字典类型查字典数据。
// 按排序号升序，前端下拉与标签渲染都依赖这个顺序。
func (r *DictDataRepository) SelectByType(ctx context.Context, dictType string) ([]*model.SysDictData, error) {
	if dictType == "" {
		return nil, nil
	}

	var rows []*model.SysDictData
	err := r.db.WithContext(ctx).Model(&model.SysDictData{}).
		Where("dict_type = ?", dictType).
		Order("dict_sort").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询字典类型 %q 的数据失败: %w", dictType, err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查字典数据。
func (r *DictDataRepository) SelectPageList(ctx context.Context, q bo.SysDictDataQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysDictData], error) {

	db := applyDictDataQuery(r.db.WithContext(ctx).Model(&model.SysDictData{}), q)
	// 仅在调用方未指定排序时兜底。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("dict_sort").Order("dict_code")
	}

	var rows []*model.SysDictData
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查字典数据，供导出等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyDictDataQuery，保证两种路径的过滤条件永不漂移。
func (r *DictDataRepository) SelectList(ctx context.Context, q bo.SysDictDataQueryBo,
	limit int) ([]*model.SysDictData, error) {

	db := applyDictDataQuery(r.db.WithContext(ctx).Model(&model.SysDictData{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定排序保证输出顺序稳定。
	db = db.Order("dict_sort").Order("dict_code")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysDictData
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询字典数据列表失败: %w", err)
	}
	return rows, nil
}

// applyDictDataQuery 应用字典数据查询条件：
// 排序号/类型走 =，标签走 LIKE；空值一概不筛。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyDictDataQuery(db *gorm.DB, q bo.SysDictDataQueryBo) *gorm.DB {
	if q.DictSort > 0 {
		db = db.Where("dict_sort = ?", q.DictSort)
	}
	if q.DictLabel != "" {
		db = db.Where("dict_label LIKE ?", "%"+escapeLike(q.DictLabel)+"%")
	}
	if q.DictType != "" {
		db = db.Where("dict_type = ?", q.DictType)
	}
	return db
}

// Insert 插入一条字典数据。
// dict_code 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *DictDataRepository) Insert(ctx context.Context, data *model.SysDictData) error {
	if data == nil {
		return fmt.Errorf("repository: 字典数据为空")
	}
	if err := r.db.WithContext(ctx).Create(data).Error; err != nil {
		return fmt.Errorf("repository: 插入字典数据失败: %w", err)
	}
	return nil
}

// UpdateByID 按字典编码更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空样式类名/备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *DictDataRepository) UpdateByID(ctx context.Context, dictCode int64,
	columns map[string]any) (int64, error) {

	if dictCode <= 0 {
		return 0, fmt.Errorf("repository: 字典数据主键无效: %d", dictCode)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 字典数据更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysDictData{}).
		Where("dict_code = ?", dictCode).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新字典数据 code=%d 失败: %w", dictCode, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateTypeByType 把某字典类型下所有数据的 dict_type 改成新值，返回受影响行数。
func (r *DictDataRepository) UpdateTypeByType(ctx context.Context,
	oldType, newType string) (int64, error) {

	if oldType == "" || newType == "" {
		return 0, fmt.Errorf("repository: 字典类型为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysDictData{}).
		Where("dict_type = ?", oldType).
		Update("dict_type", newType)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 联动更新字典类型 %q→%q 失败: %w",
			oldType, newType, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按字典编码批量删除，返回受影响行数。
// sys_dict_data 无 del_flag，这是物理删除。
func (r *DictDataRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 字典数据主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("dict_code IN ?", ids).
		Delete(&model.SysDictData{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除字典数据 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByTypeAndValue 判断同一字典类型下的键值是否已被占用，excludeID > 0 时排除该主键
// （类型 + 键值两列共同判重，供修改场景排除自身）。
func (r *DictDataRepository) ExistsByTypeAndValue(ctx context.Context, dictType,
	dictValue string, excludeID int64) (bool, error) {

	if dictType == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysDictData{}).
		Where("dict_type = ?", dictType).
		Where("dict_value = ?", dictValue)
	if excludeID > 0 {
		db = db.Where("dict_code <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验字典键值 %q 失败: %w", dictValue, err)
	}
	return count > 0, nil
}

// ExistsByType 判断某字典类型下是否已有字典数据。
func (r *DictDataRepository) ExistsByType(ctx context.Context, dictType string) (bool, error) {
	if dictType == "" {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysDictData{}).
		Where("dict_type = ?", dictType).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询字典类型 %q 的数据失败: %w", dictType, err)
	}
	return count > 0, nil
}
