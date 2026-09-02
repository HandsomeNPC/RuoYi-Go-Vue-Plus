package service

import (
	"context"
	"errors"
	"fmt"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// ErrPostNotFound 岗位不存在。
var ErrPostNotFound = errors.New("service: 岗位不存在")

// ErrPostNameExists 同一部门下的岗位名称已被占用。
var ErrPostNameExists = errors.New("service: 岗位名称已存在")

// ErrPostCodeExists 岗位编码已被占用。
var ErrPostCodeExists = errors.New("service: 岗位编码已存在")

// ErrPostHasUsers 岗位下已有分配用户，不能禁用。
var ErrPostHasUsers = errors.New("service: 该岗位下存在已分配用户，不能禁用")

// PostService 岗位业务逻辑。
type PostService struct{}

// PostSvcApp 包级实例。
var PostSvcApp = new(PostService)

// SelectPostsByUserId 按用户ID查岗位列表（对应 Java SysPostServiceImpl#selectPostsByUserId）。
func (s *PostService) SelectPostsByUserId(ctx context.Context, userID int64) ([]*vo.SysPostVo, error) {
	posts, err := repository.NewPostRepository(database.DB()).SelectPostsByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysPostVoList(posts), nil
}

// QueryPageList 按条件分页查岗位。BelongDeptID 会在此时解析成部门ID集。
func (s *PostService) QueryPageList(ctx context.Context, q bo.SysPostQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysPostVo], error) {

	if err := s.resolveDeptIDs(ctx, &q); err != nil {
		return pkgrepo.EmptyPage[*vo.SysPostVo](), err
	}

	res, err := repository.NewPostRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysPostVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysPostVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查岗位，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *PostService) QueryList(ctx context.Context, q bo.SysPostQueryBo,
	limit int) ([]*vo.SysPostVo, error) {

	if err := s.resolveDeptIDs(ctx, &q); err != nil {
		return nil, err
	}
	rows, err := repository.NewPostRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysPostVoList(rows), nil
}

// QueryByID 按主键查岗位（对应 Java selectPostById），不存在时返回 ErrPostNotFound。
func (s *PostService) QueryByID(ctx context.Context, postID int64) (*vo.SysPostVo, error) {
	post, err := repository.NewPostRepository(database.DB()).SelectByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysPostVo(post), nil
}

// OptionSelect 岗位选择框列表（对应 Java optionselect）。
// deptID > 0 时返回该部门下的全部岗位；否则按 postIDs 返回启用岗位（postIDs 为空则返回全部启用岗位）。
func (s *PostService) OptionSelect(ctx context.Context, deptID int64,
	postIDs []int64) ([]*vo.SysPostVo, error) {

	repo := repository.NewPostRepository(database.DB())
	if deptID > 0 {
		rows, err := repo.SelectList(ctx, bo.SysPostQueryBo{DeptID: deptID}, 0)
		if err != nil {
			return nil, err
		}
		return vo.Conv.ConvertToSysPostVoList(rows), nil
	}
	rows, err := repo.SelectNormalByIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysPostVoList(rows), nil
}

// CheckPostNameUnique 校验同一部门下的岗位名称是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *PostService) CheckPostNameUnique(ctx context.Context, postName string,
	deptID, excludeID int64) (bool, error) {

	exists, err := repository.NewPostRepository(database.DB()).
		ExistsByPostName(ctx, postName, deptID, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckPostCodeUnique 校验岗位编码是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *PostService) CheckPostCodeUnique(ctx context.Context, postCode string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewPostRepository(database.DB()).
		ExistsByPostCode(ctx, postCode, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CountUserPostByID 统计已分配该岗位的用户数（对应 Java countUserPostById）。
func (s *PostService) CountUserPostByID(ctx context.Context, postID int64) (int64, error) {
	return repository.NewPostRepository(database.DB()).CountUserPostByID(ctx, postID)
}

// InsertPost 新增岗位（对应 Java insertPost）。名称/编码重复时返回对应哨兵错误。
func (s *PostService) InsertPost(ctx context.Context, b *bo.SysPostBo) error {
	if b == nil {
		return errors.New("service: 岗位入参为空")
	}

	if unique, err := s.CheckPostNameUnique(ctx, b.PostName, b.DeptID, 0); err != nil {
		return err
	} else if !unique {
		return ErrPostNameExists
	}
	if unique, err := s.CheckPostCodeUnique(ctx, b.PostCode, 0); err != nil {
		return err
	} else if !unique {
		return ErrPostCodeExists
	}

	add := bo.Conv.ConvertToSysPost(b)
	add.PostID = snowflake.Next() // post_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewPostRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.PostID = add.PostID
	return nil
}

// UpdatePost 修改岗位（对应 Java updatePost）。
//
// 名称/编码重复与「禁用但已有用户」三道校验对齐 Java edit 的 if-else 链，
// 同时触发多条时前端看到的提示才与 Java 一致。
func (s *PostService) UpdatePost(ctx context.Context, b *bo.SysPostBo) error {
	if b == nil {
		return errors.New("service: 岗位入参为空")
	}
	if b.PostID <= 0 {
		return errors.New("service: 岗位主键不能为空")
	}

	if unique, err := s.CheckPostNameUnique(ctx, b.PostName, b.DeptID, b.PostID); err != nil {
		return err
	} else if !unique {
		return ErrPostNameExists
	}
	if unique, err := s.CheckPostCodeUnique(ctx, b.PostCode, b.PostID); err != nil {
		return err
	} else if !unique {
		return ErrPostCodeExists
	}
	// 停用前确认没有用户引用：否则会让已分配该岗位的用户挂在停用岗位上。
	if b.Status == constant.StatusDisable {
		count, err := s.CountUserPostByID(ctx, b.PostID)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrPostHasUsers
		}
	}

	repo := repository.NewPostRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"岗位不存在"。
	if _, err := repo.SelectByID(ctx, b.PostID); err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrPostNotFound
		}
		return err
	}

	if _, err := repo.UpdateByID(ctx, b.PostID, buildPostUpdateColumns(b)); err != nil {
		return err
	}
	return nil
}

// buildPostUpdateColumns 组装修改岗位的更新列。
func buildPostUpdateColumns(b *bo.SysPostBo) map[string]any {
	columns := map[string]any{
		"dept_id":       b.DeptID,
		"post_code":     b.PostCode,
		"post_name":     b.PostName,
		"post_category": b.PostCategory,
		"post_sort":     b.PostSort,
		// 一律写入，让前端能把备注清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"remark": b.Remark,
	}
	// 状态缺省即视为不改：漏传字段不该把线上的 '0' 刷成空串，
	// 那会让岗位既不算启用也不算停用。等效于 Java updateById 对 null 字段的跳过。
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// DeletePostByIDs 批量删除岗位（对应 Java deletePostByIds）。
// 任一岗位已有用户分配即整批拒绝，不做部分删除。
func (s *PostService) DeletePostByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 岗位主键不能为空")
	}

	repo := repository.NewPostRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	// 先整批校验再删，不边删边校验：Java 侧靠抛异常回滚事务，这里没有事务包裹，
	// 一旦先删了几行再撞上已分配的岗位就会留下删一半的状态。
	for _, post := range rows {
		count, err := repo.CountUserPostByID(ctx, post.PostID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errs.New(0, fmt.Sprintf("%s已分配，不能删除", post.PostName), "")
		}
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	return nil
}

// resolveDeptIDs 把 BelongDeptID 解析成「自身+全部子部门」的ID集，写回 q.DeptIDs
// 供 repository 的 IN 过滤用。单部门搜索（DeptID）优先时不解析。
func (s *PostService) resolveDeptIDs(ctx context.Context, q *bo.SysPostQueryBo) error {
	if q.DeptID > 0 || q.BelongDeptID <= 0 {
		return nil
	}
	ids, err := repository.NewDeptRepository(database.DB()).
		SelectDeptAndChildIDs(ctx, q.BelongDeptID)
	if err != nil {
		return err
	}
	q.DeptIDs = ids
	return nil
}

// SelectPostListByUserId 按用户ID取已授权岗位ID列表（对应 Java selectPostListByUserId）。
func (s *PostService) SelectPostListByUserId(ctx context.Context, userID int64) ([]int64, error) {
	posts, err := repository.NewPostRepository(database.DB()).SelectPostsByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(posts))
	for _, p := range posts {
		if p != nil {
			out = append(out, p.PostID)
		}
	}
	return out, nil
}
