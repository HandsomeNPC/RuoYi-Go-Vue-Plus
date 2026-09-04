package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/pkg/constant"
)

var ErrDeptNotFound = errors.New("repository: 部门不存在")

// DeptRepository sys_dept 数据访问。
type DeptRepository struct {
	db *gorm.DB
}

// NewDeptRepository 构造部门 repository。
func NewDeptRepository(db *gorm.DB) *DeptRepository {
	return &DeptRepository{db: db}
}

// SelectByID 按主键查部门，不存在时返回 ErrDeptNotFound。
// 仅取实体本身；父部门名等扩展字段由 service 层按需回填，buildLoginUser 只读 DeptName/DeptCategory。
func (r *DeptRepository) SelectByID(ctx context.Context, deptID int64) (*model.SysDept, error) {
	if deptID <= 0 {
		return nil, ErrDeptNotFound
	}

	var dept model.SysDept
	err := r.db.WithContext(ctx).
		Where("dept_id = ?", deptID).
		First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeptNotFound
		}
		return nil, fmt.Errorf("repository: 查询部门 id=%d 失败: %w", deptID, err)
	}
	return &dept, nil
}

// SelectList 按条件查部门列表。
// 部门总量有限（前端按树整体渲染、无分页），故不分页也不限行数。
func (r *DeptRepository) SelectList(ctx context.Context, q bo.SysDeptQueryBo) ([]*model.SysDept, error) {
	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	db := applyDeptQuery(r.db.WithContext(ctx).Model(&model.SysDept{}), q)
	// 祖先链在前保证父节点先于子节点出现，前端 listToTree 与 tree.BuildInPlace 都依赖这个顺序。
	db = db.Order("ancestors").Order("parent_id").Order("order_num").Order("dept_id")

	var rows []*model.SysDept
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询部门列表失败: %w", err)
	}
	return rows, nil
}

// applyDeptQuery 应用部门查询条件：
// 名称/类别编码走 LIKE，主键/父级/状态走 =，创建时间走闭区间。
func applyDeptQuery(db *gorm.DB, q bo.SysDeptQueryBo) *gorm.DB {
	if q.DeptID > 0 {
		db = db.Where("dept_id = ?", q.DeptID)
	}
	if q.ParentID > 0 {
		db = db.Where("parent_id = ?", q.ParentID)
	}
	if q.DeptName != "" {
		db = db.Where("dept_name LIKE ?", "%"+escapeLike(q.DeptName)+"%")
	}
	if q.DeptCategory != "" {
		db = db.Where("dept_category LIKE ?", "%"+escapeLike(q.DeptCategory)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 两端须同时给出：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	// 部门树搜索：命中该部门自身及其全部后代。
	//
	// 直接展开成「自身 OR 祖先链包含它」——语义等价于先查子部门主键再 IN，
	// 但少一次往返，且不必把子部门主键搬进 SQL 参数。
	if q.BelongDeptID > 0 {
		db = db.Where("(dept_id = ? OR FIND_IN_SET(?, ancestors) > 0)", q.BelongDeptID, q.BelongDeptID)
	}
	return db
}

// SelectNormalByIDs 查启用状态的部门，ids 非空时按主键过滤
// （ids 为空即不加 IN 条件，退化成查全部启用部门）。
func (r *DeptRepository) SelectNormalByIDs(ctx context.Context, ids []int64) ([]*model.SysDept, error) {
	db := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("status = ?", constant.StatusNormal)
	if len(ids) > 0 {
		db = db.Where("dept_id IN ?", ids)
	}
	// 固定按主键升序：下拉框的选项顺序不该随 MySQL 的返回顺序漂移。
	db = db.Order("dept_id")

	var rows []*model.SysDept
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询部门选择框列表失败: %w", err)
	}
	return rows, nil
}

// SelectDeptAndChildIDs 取部门自身及其全部后代的ID集合
// （供岗位按部门树搜索时 IN 过滤）。
// 命中条件：自身 dept_id 命中，或祖先链 FIND_IN_SET 命中。
func (r *DeptRepository) SelectDeptAndChildIDs(ctx context.Context,
	deptID int64) ([]int64, error) {

	if deptID <= 0 {
		return nil, nil
	}

	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("dept_id = ? OR FIND_IN_SET(?, ancestors) > 0", deptID, deptID).
		Pluck("dept_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询部门 %d 及子部门ID失败: %w", deptID, err)
	}
	return ids, nil
}

// SelectChildrenByAncestor 查祖先链包含 deptID 的全部后代。
func (r *DeptRepository) SelectChildrenByAncestor(ctx context.Context,
	deptID int64) ([]*model.SysDept, error) {

	if deptID <= 0 {
		return nil, nil
	}

	var rows []*model.SysDept
	err := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("FIND_IN_SET(?, ancestors) > 0", deptID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 查询部门 %d 的子部门失败: %w", deptID, err)
	}
	return rows, nil
}

// CountNormalChildren 统计 deptID 名下处于启用状态的后代数量。
func (r *DeptRepository) CountNormalChildren(ctx context.Context, deptID int64) (int64, error) {
	if deptID <= 0 {
		return 0, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("status = ?", constant.StatusNormal).
		Where("FIND_IN_SET(?, ancestors) > 0", deptID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("repository: 统计部门 %d 的启用子部门失败: %w", deptID, err)
	}
	return count, nil
}

// ExistsByParentID 判断是否存在以 deptID 为直接上级的部门。
func (r *DeptRepository) ExistsByParentID(ctx context.Context, deptID int64) (bool, error) {
	if deptID <= 0 {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("parent_id = ?", deptID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询部门 %d 的下级部门失败: %w", deptID, err)
	}
	return count > 0, nil
}

// ExistsByDeptName 判断同一上级下的部门名称是否已被占用，excludeID > 0 时排除该主键
// （名称 + 父级两列共同判重，供修改场景排除自身）。
func (r *DeptRepository) ExistsByDeptName(ctx context.Context, deptName string,
	parentID, excludeID int64) (bool, error) {

	if deptName == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysDept{}).
		Where("dept_name = ?", deptName).
		Where("parent_id = ?", parentID)
	if excludeID > 0 {
		db = db.Where("dept_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验部门名称 %q 失败: %w", deptName, err)
	}
	return count > 0, nil
}

// Insert 插入一条部门。
// dept_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *DeptRepository) Insert(ctx context.Context, dept *model.SysDept) error {
	if dept == nil {
		return fmt.Errorf("repository: 部门为空")
	}
	if err := r.db.WithContext(ctx).Create(dept).Error; err != nil {
		return fmt.Errorf("repository: 插入部门失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在或已被逻辑删除）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空联系电话/邮箱」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *DeptRepository) UpdateByID(ctx context.Context, deptID int64,
	columns map[string]any) (int64, error) {

	if deptID <= 0 {
		return 0, fmt.Errorf("repository: 部门主键无效: %d", deptID)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 部门更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysDept{}).
		Where("dept_id = ?", deptID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新部门 id=%d 失败: %w", deptID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateStatusNormalByIDs 把指定部门批量置为启用。
func (r *DeptRepository) UpdateStatusNormalByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysDept{}).
		Where("dept_id IN ?", ids).
		Update("status", constant.StatusNormal)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 启用部门 %v 失败: %w", ids, res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteByID 按主键删除，返回受影响行数（0 表示主键不存在或已被删除）。
//
// SysDept 嵌了 LogicDelete，Delete 会被改写成 UPDATE ... SET del_flag = '1'，
// 并自动带上 del_flag = '0'，重复删除同一主键第二次即返回 0。
func (r *DeptRepository) DeleteByID(ctx context.Context, deptID int64) (int64, error) {
	if deptID <= 0 {
		return 0, fmt.Errorf("repository: 部门主键无效: %d", deptID)
	}

	res := r.db.WithContext(ctx).
		Where("dept_id = ?", deptID).
		Delete(&model.SysDept{})
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 删除部门 %d 失败: %w", deptID, res.Error)
	}
	return res.RowsAffected, nil
}
