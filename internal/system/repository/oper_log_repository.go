package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
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

// SelectPageList 按条件分页查操作日志。
func (r *OperLogRepository) SelectPageList(ctx context.Context, q bo.SysOperLogQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysOperLog], error) {

	db := applyOperLogQuery(r.db.WithContext(ctx).Model(&model.SysOperLog{}), q)
	// 仅在调用方未指定排序时按主键降序兜底。
	if !page.HasOrder() {
		db = db.Order("oper_id DESC")
	}

	var rows []*model.SysOperLog
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查操作日志，供导出等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyOperLogQuery，保证两种路径的过滤条件永不漂移。
func (r *OperLogRepository) SelectList(ctx context.Context, q bo.SysOperLogQueryBo,
	limit int) ([]*model.SysOperLog, error) {

	db := applyOperLogQuery(r.db.WithContext(ctx).Model(&model.SysOperLog{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定按主键降序，保证输出顺序稳定。
	db = db.Order("oper_id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysOperLog
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询操作日志列表失败: %w", err)
	}
	return rows, nil
}

// applyOperLogQuery 应用操作日志查询条件：
// operIp/title/operName/browser/os 走 like，businessType/status/userId/deptId/clientKey/deviceType
// 走 eq，businessTypes 走 in，oper_time 走闭区间；空串/零值一概不筛。
// escapeLike 复用 config_repository.go 同包定义。
func applyOperLogQuery(db *gorm.DB, q bo.SysOperLogQueryBo) *gorm.DB {
	if q.OperIP != "" {
		db = db.Where("oper_ip LIKE ?", "%"+escapeLike(q.OperIP)+"%")
	}
	if q.Title != "" {
		db = db.Where("title LIKE ?", "%"+escapeLike(q.Title)+"%")
	}
	// 0=其他 不参与单值过滤，要按"其他"筛走 BusinessTypes。
	if q.BusinessType > 0 {
		db = db.Where("business_type = ?", q.BusinessType)
	}
	if len(q.BusinessTypes) > 0 {
		db = db.Where("business_type IN ?", q.BusinessTypes)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.OperName != "" {
		db = db.Where("oper_name LIKE ?", "%"+escapeLike(q.OperName)+"%")
	}
	if q.UserID > 0 {
		db = db.Where("user_id = ?", q.UserID)
	}
	if q.DeptID > 0 {
		db = db.Where("dept_id = ?", q.DeptID)
	}
	if q.ClientKey != "" {
		db = db.Where("client_key = ?", q.ClientKey)
	}
	if q.DeviceType != "" {
		db = db.Where("device_type = ?", q.DeviceType)
	}
	if q.Browser != "" {
		db = db.Where("browser LIKE ?", "%"+escapeLike(q.Browser)+"%")
	}
	if q.OS != "" {
		db = db.Where("os LIKE ?", "%"+escapeLike(q.OS)+"%")
	}
	// 两端须同时给出：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("oper_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	return db
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_oper_log 无 del_flag，这是物理删除。
func (r *OperLogRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 操作日志主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("oper_id IN ?", ids).
		Delete(&model.SysOperLog{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除操作日志 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// Clean 清空全部操作日志。
func (r *OperLogRepository) Clean(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Where("1 = 1").Delete(&model.SysOperLog{}).Error; err != nil {
		return fmt.Errorf("repository: 清空操作日志失败: %w", err)
	}
	return nil
}
