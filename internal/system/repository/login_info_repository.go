package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
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

// SelectPageList 按条件分页查登录日志。
func (r *LoginInfoRepository) SelectPageList(ctx context.Context, q bo.SysLoginInfoQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysLoginInfo], error) {

	db := applyLoginInfoQuery(r.db.WithContext(ctx).Model(&model.SysLoginInfo{}), q)
	// 仅在调用方未指定排序时按主键降序兜底（对齐 Java orderByDesc(SysLoginInfo::getInfoId)）。
	if !page.HasOrder() {
		db = db.Order("info_id DESC")
	}

	var rows []*model.SysLoginInfo
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查登录日志，供导出等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyLoginInfoQuery，保证两种路径的过滤条件永不漂移。
func (r *LoginInfoRepository) SelectList(ctx context.Context, q bo.SysLoginInfoQueryBo,
	limit int) ([]*model.SysLoginInfo, error) {

	db := applyLoginInfoQuery(r.db.WithContext(ctx).Model(&model.SysLoginInfo{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定按主键降序，保证输出顺序稳定。
	db = db.Order("info_id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysLoginInfo
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询登录日志列表失败: %w", err)
	}
	return rows, nil
}

// applyLoginInfoQuery 应用登录日志查询条件（对齐 Java buildQueryWrapper）：
// ipaddr/userName 走 like，status 走 eq，login_time 走闭区间；空串一概不筛。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
// escapeLike 复用 config_repository.go 同包定义。
func applyLoginInfoQuery(db *gorm.DB, q bo.SysLoginInfoQueryBo) *gorm.DB {
	if q.IPAddr != "" {
		db = db.Where("ipaddr LIKE ?", "%"+escapeLike(q.IPAddr)+"%")
	}
	if q.UserName != "" {
		db = db.Where("user_name LIKE ?", "%"+escapeLike(q.UserName)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("login_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	return db
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_login_info 无 del_flag，这是物理删除（对齐 Java deleteByIds）。
func (r *LoginInfoRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 登录日志主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("info_id IN ?", ids).
		Delete(&model.SysLoginInfo{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除登录日志 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// Clean 清空全部登录日志（对应 Java cleanLoginInfo 的 lambda().delete()）。
func (r *LoginInfoRepository) Clean(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Where("1 = 1").Delete(&model.SysLoginInfo{}).Error; err != nil {
		return fmt.Errorf("repository: 清空登录日志失败: %w", err)
	}
	return nil
}
