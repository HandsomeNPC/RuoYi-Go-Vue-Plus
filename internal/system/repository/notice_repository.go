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

var ErrNoticeNotFound = errors.New("repository: 通知公告不存在")

// NoticeRepository sys_notice 数据访问。
type NoticeRepository struct {
	db *gorm.DB
}

// NewNoticeRepository 构造通知公告 repository。
func NewNoticeRepository(db *gorm.DB) *NoticeRepository {
	return &NoticeRepository{db: db}
}

// SelectByID 按主键查通知公告，不存在时返回 ErrNoticeNotFound。
func (r *NoticeRepository) SelectByID(ctx context.Context, noticeID int64) (*model.SysNotice, error) {
	if noticeID <= 0 {
		return nil, ErrNoticeNotFound
	}

	var notice model.SysNotice
	err := r.db.WithContext(ctx).
		Where("notice_id = ?", noticeID).
		First(&notice).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoticeNotFound
		}
		return nil, fmt.Errorf("repository: 查询通知公告 id=%d 失败: %w", noticeID, err)
	}
	return &notice, nil
}

// SelectPageList 按条件分页查通知公告。
// createBy > 0 时按创建者过滤，由 service 从创建人账号解析而来。
func (r *NoticeRepository) SelectPageList(ctx context.Context, q bo.SysNoticeQueryBo,
	createBy int64, page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysNotice], error) {

	db := applyNoticeQuery(r.db.WithContext(ctx).Model(&model.SysNotice{}), q, createBy)
	// 仅在调用方未指定排序时按主键升序兜底。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("notice_id")
	}

	var rows []*model.SysNotice
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查通知公告。
// limit <= 0 表示不限制。与 SelectPageList 共用 applyNoticeQuery，保证过滤条件不漂移。
func (r *NoticeRepository) SelectList(ctx context.Context, q bo.SysNoticeQueryBo,
	createBy int64, limit int) ([]*model.SysNotice, error) {

	db := applyNoticeQuery(r.db.WithContext(ctx).Model(&model.SysNotice{}), q, createBy)
	// 非翻页场景没有"调用方另指定排序"一说，固定按主键升序，保证输出顺序稳定。
	db = db.Order("notice_id")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysNotice
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询通知公告列表失败: %w", err)
	}
	return rows, nil
}

// applyNoticeQuery 应用通知公告查询条件：
// 标题走 like，类型与创建者走 eq；空值一概不筛。
// 分页与列表两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyNoticeQuery(db *gorm.DB, q bo.SysNoticeQueryBo, createBy int64) *gorm.DB {
	if q.NoticeTitle != "" {
		db = db.Where("notice_title LIKE ?", "%"+escapeLike(q.NoticeTitle)+"%")
	}
	if q.NoticeType != "" {
		db = db.Where("notice_type = ?", q.NoticeType)
	}
	// 0 表示不按创建者筛。非 0 一律落条件，包括 service 传来的负数哨兵
	// （创建人账号查不到用户时用它，让结果为空而非退化成返回全部公告）。
	if createBy != 0 {
		db = db.Where("create_by = ?", createBy)
	}
	return db
}

// Insert 插入一条通知公告。
// notice_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *NoticeRepository) Insert(ctx context.Context, notice *model.SysNotice) error {
	if notice == nil {
		return fmt.Errorf("repository: 通知公告为空")
	}
	if err := r.db.WithContext(ctx).Create(notice).Error; err != nil {
		return fmt.Errorf("repository: 插入通知公告失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *NoticeRepository) UpdateByID(ctx context.Context, noticeID int64,
	columns map[string]any) (int64, error) {

	if noticeID <= 0 {
		return 0, fmt.Errorf("repository: 通知公告主键无效: %d", noticeID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 通知公告更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysNotice{}).
		Where("notice_id = ?", noticeID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新通知公告 id=%d 失败: %w", noticeID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_notice 无 del_flag，这是物理删除。
func (r *NoticeRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 通知公告主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("notice_id IN ?", ids).
		Delete(&model.SysNotice{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除通知公告 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}
