package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/resource/model"
	"ruoyi-go-vue-plus/internal/resource/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrOssNotFound OSS 对象不存在。
var ErrOssNotFound = errors.New("repository: OSS 对象不存在")

// OssRepository sys_oss 数据访问。
type OssRepository struct {
	db *gorm.DB
}

// NewOssRepository 构造 OSS repository。
func NewOssRepository(db *gorm.DB) *OssRepository {
	return &OssRepository{db: db}
}

// SelectByID 按主键查 OSS 对象，不存在时返回 ErrOssNotFound。
func (r *OssRepository) SelectByID(ctx context.Context, ossID int64) (*model.SysOss, error) {
	if ossID <= 0 {
		return nil, ErrOssNotFound
	}

	var oss model.SysOss
	err := r.db.WithContext(ctx).Where("oss_id = ?", ossID).First(&oss).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOssNotFound
		}
		return nil, fmt.Errorf("repository: 查询 OSS 对象 id=%d 失败: %w", ossID, err)
	}
	return &oss, nil
}

// SelectByIDs 按主键批量查，返回实际命中的行（缺失主键静默跳过，由调用方比对数量）。
func (r *OssRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysOss, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysOss
	if err := r.db.WithContext(ctx).Where("oss_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询 OSS 对象 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查 OSS 对象。
func (r *OssRepository) SelectPageList(ctx context.Context, q bo.SysOssQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysOss], error) {

	db := applyOssQuery(r.db.WithContext(ctx).Model(&model.SysOss{}), q)
	// 仅在调用方未指定排序时按主键升序兜底（对齐 Java orderByAsc(SysOss::getOssId)）。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("oss_id")
	}

	var rows []*model.SysOss
	return pkgrepo.SelectPage(db, page, &rows)
}

// applyOssQuery 应用 OSS 查询条件（对齐 Java buildQueryWrapper）：
// 文件名/原名走 like，后缀/URL/服务商走 eq，上传人走 eq，上传时间走闭区间；空值一概不筛。
func applyOssQuery(db *gorm.DB, q bo.SysOssQueryBo) *gorm.DB {
	if q.FileName != "" {
		db = db.Where("file_name LIKE ?", "%"+escapeLike(q.FileName)+"%")
	}
	if q.OriginalName != "" {
		db = db.Where("original_name LIKE ?", "%"+escapeLike(q.OriginalName)+"%")
	}
	if q.FileSuffix != "" {
		db = db.Where("file_suffix = ?", q.FileSuffix)
	}
	if q.URL != "" {
		db = db.Where("url = ?", q.URL)
	}
	if q.Service != "" {
		db = db.Where("service = ?", q.Service)
	}
	// 0 视为不筛：对齐 Java eqIfPresent 对 null 的处理，且不存在 user_id 为 0 的上传人。
	if q.CreateBy > 0 {
		db = db.Where("create_by = ?", q.CreateBy)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginCreateTime != "" && q.EndCreateTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginCreateTime, q.EndCreateTime)
	}
	return db
}

// Insert 插入一条 OSS 记录。
// oss_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *OssRepository) Insert(ctx context.Context, oss *model.SysOss) error {
	if oss == nil {
		return fmt.Errorf("repository: OSS 对象为空")
	}
	if err := r.db.WithContext(ctx).Create(oss).Error; err != nil {
		return fmt.Errorf("repository: 插入 OSS 对象失败: %w", err)
	}
	return nil
}

// DeleteByIDs 按主键批量删除，返回受影响行数。
// sys_oss 无 del_flag，这是物理删除（对齐 Java 侧该表未开逻辑删除）。
func (r *OssRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: OSS 对象主键为空")
	}

	res := r.db.WithContext(ctx).Where("oss_id IN ?", ids).Delete(&model.SysOss{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除 OSS 对象 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}
