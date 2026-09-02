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

// ErrPostNotFound 岗位不存在。
var ErrPostNotFound = errors.New("repository: 岗位不存在")

// PostRepository sys_post 数据访问。
type PostRepository struct {
	db *gorm.DB
}

// NewPostRepository 构造岗位 repository。
func NewPostRepository(db *gorm.DB) *PostRepository {
	return &PostRepository{db: db}
}

// CountByDeptID 统计部门下的岗位数（对应 Java countPostByDeptId）。
func (r *PostRepository) CountByDeptID(ctx context.Context, deptID int64) (int64, error) {
	if deptID <= 0 {
		return 0, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysPost{}).
		Where("dept_id = ?", deptID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("repository: 统计部门 %d 下的岗位失败: %w", deptID, err)
	}
	return count, nil
}

// SelectPostsByUserId 按用户ID查其关联岗位（对应 Java SysPostMapper#selectPostsByUserId）。
// 经 sys_user_post 关联，sys_post 的逻辑删除由实体自动过滤；不按岗位状态过滤（与 Java 一致）。
func (r *PostRepository) SelectPostsByUserId(ctx context.Context, userID int64) ([]*model.SysPost, error) {
	if userID <= 0 {
		return nil, nil
	}

	var posts []*model.SysPost
	err := r.db.WithContext(ctx).
		Joins("JOIN sys_user_post sup ON sup.post_id = sys_post.post_id").
		Where("sup.user_id = ?", userID).
		Order("sys_post.post_sort").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询用户 %d 岗位失败: %w", userID, err)
	}
	return posts, nil
}

// SelectByID 按主键查岗位，不存在时返回 ErrPostNotFound。
func (r *PostRepository) SelectByID(ctx context.Context, postID int64) (*model.SysPost, error) {
	if postID <= 0 {
		return nil, ErrPostNotFound
	}

	var post model.SysPost
	err := r.db.WithContext(ctx).Where("post_id = ?", postID).First(&post).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("repository: 查询岗位 id=%d 失败: %w", postID, err)
	}
	return &post, nil
}

// SelectByIDs 按主键批量查，返回实际命中的行（缺失主键静默跳过，由调用方比对数量）。
func (r *PostRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysPost, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []*model.SysPost
	if err := r.db.WithContext(ctx).Where("post_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询岗位 %v 失败: %w", ids, err)
	}
	return rows, nil
}

// SelectPageList 按条件分页查岗位（对应 Java selectPagePostList）。
func (r *PostRepository) SelectPageList(ctx context.Context, q bo.SysPostQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysPost], error) {

	db := applyPostQuery(r.db.WithContext(ctx).Model(&model.SysPost{}), q)
	// 仅在调用方未指定排序时按 post_sort 升序兜底（对齐 Java orderByAsc(SysPost::getPostSort)）。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("post_sort")
	}

	var rows []*model.SysPost
	return pkgrepo.SelectPage(db, page, &rows)
}

// SelectList 按条件不分页查岗位，供导出与下拉选择等需要全量的场景用。
// limit <= 0 表示不限制；超限由调用方通过多取一行来判定，避免先捞完再拒绝。
//
// 与 SelectPageList 共用 applyPostQuery，保证两种路径的过滤条件永不漂移。
func (r *PostRepository) SelectList(ctx context.Context, q bo.SysPostQueryBo,
	limit int) ([]*model.SysPost, error) {

	db := applyPostQuery(r.db.WithContext(ctx).Model(&model.SysPost{}), q)
	// 导出不是翻页，没有"调用方另指定排序"一说，固定按 post_sort 升序，保证输出顺序稳定。
	db = db.Order("post_sort")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysPost
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询岗位列表失败: %w", err)
	}
	return rows, nil
}

// SelectNormalByIDs 查启用状态的岗位，ids 非空时按主键过滤
// （对应 Java selectPostByIds：ids 为空即不加 IN 条件，退化成查全部启用岗位）。
func (r *PostRepository) SelectNormalByIDs(ctx context.Context, ids []int64) ([]*model.SysPost, error) {
	db := r.db.WithContext(ctx).Model(&model.SysPost{}).Where("status = ?", "0")
	if len(ids) > 0 {
		db = db.Where("post_id IN ?", ids)
	}
	// Java 未指定排序，这里固定按 post_sort 升序：下拉框的选项顺序不该随 MySQL 返回顺序漂移。
	db = db.Order("post_sort")

	var rows []*model.SysPost
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询岗位选择框列表失败: %w", err)
	}
	return rows, nil
}

// applyPostQuery 应用岗位查询条件（对齐 Java buildQueryWrapper）：
// 编码/类别/名称走 LIKE，状态走 =，创建时间走闭区间，部门走单部门 eq 或部门树 IN。
// 分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyPostQuery(db *gorm.DB, q bo.SysPostQueryBo) *gorm.DB {
	if q.PostCode != "" {
		db = db.Where("post_code LIKE ?", "%"+escapeLike(q.PostCode)+"%")
	}
	if q.PostCategory != "" {
		db = db.Where("post_category LIKE ?", "%"+escapeLike(q.PostCategory)+"%")
	}
	if q.PostName != "" {
		db = db.Where("post_name LIKE ?", "%"+escapeLike(q.PostName)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	// 单部门搜索优先于部门树搜索（对齐 Java 的 if/else if）。
	if q.DeptID > 0 {
		db = db.Where("dept_id = ?", q.DeptID)
	} else if len(q.DeptIDs) > 0 {
		// 部门树搜索：DeptIDs 由 service 按 BelongDeptID 解析成「自身+全部子部门」。
		db = db.Where("dept_id IN ?", q.DeptIDs)
	}
	return db
}

// Insert 插入一条岗位。
// post_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *PostRepository) Insert(ctx context.Context, post *model.SysPost) error {
	if post == nil {
		return fmt.Errorf("repository: 岗位为空")
	}
	if err := r.db.WithContext(ctx).Create(post).Error; err != nil {
		return fmt.Errorf("repository: 插入岗位失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空备注」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *PostRepository) UpdateByID(ctx context.Context, postID int64,
	columns map[string]any) (int64, error) {

	if postID <= 0 {
		return 0, fmt.Errorf("repository: 岗位主键无效: %d", postID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 岗位更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysPost{}).
		Where("post_id = ?", postID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新岗位 id=%d 失败: %w", postID, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByIDs 按主键批量逻辑删除，返回受影响行数。
// SysPost 嵌了 LogicDelete，Delete 会被改写成 UPDATE ... SET del_flag = '1'。
func (r *PostRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("repository: 岗位主键为空")
	}

	res := r.db.WithContext(ctx).
		Where("post_id IN ?", ids).
		Delete(&model.SysPost{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除岗位 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByPostName 判断同一部门下的岗位名称是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkPostNameUnique：名称 + 部门两列共同判重，供修改场景排除自身）。
func (r *PostRepository) ExistsByPostName(ctx context.Context, postName string,
	deptID, excludeID int64) (bool, error) {

	if postName == "" || deptID <= 0 {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysPost{}).
		Where("post_name = ?", postName).
		Where("dept_id = ?", deptID)
	if excludeID > 0 {
		db = db.Where("post_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验岗位名称 %q 失败: %w", postName, err)
	}
	return count > 0, nil
}

// ExistsByPostCode 判断岗位编码是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkPostCodeUnique：编码全局判重，供修改场景排除自身）。
func (r *PostRepository) ExistsByPostCode(ctx context.Context, postCode string,
	excludeID int64) (bool, error) {

	if postCode == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysPost{}).Where("post_code = ?", postCode)
	if excludeID > 0 {
		db = db.Where("post_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验岗位编码 %q 失败: %w", postCode, err)
	}
	return count > 0, nil
}

// CountUserPostByID 统计已分配该岗位的用户数（对应 Java countUserPostById）。
// 走 sys_user_post 关联表，岗位被引用即不可删/不可禁用。
func (r *PostRepository) CountUserPostByID(ctx context.Context, postID int64) (int64, error) {
	if postID <= 0 {
		return 0, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysUserPost{}).
		Where("post_id = ?", postID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("repository: 统计岗位 %d 的用户数失败: %w", postID, err)
	}
	return count, nil
}

// CountByIDs 按主键集合统计存在的岗位数（对应 Java selectPostCount）。
//
// Java 此方法带 @DataPermission，非超管账号下注入岗位数据隔离条件，
// 使"我能看到的岗位"与传入数量不等时抛"没有权限访问岗位的数据"。Go 侧数据权限尚未落地，
// 此处只按主键计数——等数据权限落地后挂上过滤，调用点不必改。
func (r *PostRepository) CountByIDs(ctx context.Context, postIDs []int64) (int64, error) {
	if len(postIDs) == 0 {
		return 0, nil
	}
	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SysPost{}).
		Where("post_id IN ?", postIDs).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("repository: 统计岗位 %v 失败: %w", postIDs, err)
	}
	return count, nil
}
