package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
	"ruoyi-go-vue-plus/pkg/snowflake"
	"ruoyi-go-vue-plus/pkg/tree"
)

// ErrDeptNotFound 部门不存在。
var ErrDeptNotFound = errors.New("service: 部门不存在")

// ErrDeptNameExists 同一上级下的部门名称已被占用。
var ErrDeptNameExists = errors.New("service: 部门名称已存在")

// ErrDeptParentIsSelf 上级部门指向了自己。
var ErrDeptParentIsSelf = errors.New("service: 上级部门不能是自己")

// DeptService 部门业务逻辑。
type DeptService struct{}

// DeptSvcApp 包级实例。
var DeptSvcApp = new(DeptService)

// SelectByID 按主键查部门（对应 Java SysDeptServiceImpl#selectDeptById
// + @Cacheable(key = "#deptId")），回填父部门名称。
//
// 不存在时返回 (nil, nil)：调用方语义各异——登录时按空串兜底（对齐 Java
// Opt.orElse(StringUtils.EMPTY)），详情接口回 data: null（对齐 Java R.ok(null)）。
// 这个 nil 不入缓存：查不到多半是主键写错或已被删，缓存下来只会让后续新增同 id 的
// 部门读到空值，而 Java 侧的 Redis CacheManager 同样跳过 null。
func (s *DeptService) SelectByID(ctx context.Context, deptID int64) (*vo.SysDeptVo, error) {
	var cached vo.SysDeptVo
	if hit, _ := cache.Get(ctx, constant.CacheSysDept, deptCacheKey(deptID), &cached); hit {
		return &cached, nil
	}

	repo := repository.NewDeptRepository(database.DB())
	dept, err := repo.SelectByID(ctx, deptID)
	if err != nil {
		if errors.Is(err, repository.ErrDeptNotFound) {
			return nil, nil
		}
		return nil, err
	}

	out := vo.Conv.ConvertToSysDeptVo(dept)
	// 父部门名单独查一趟；父级不存在（如根部门的 parent_id = 0）时留空，不算错。
	if parent, err := repo.SelectByID(ctx, dept.ParentID); err == nil {
		out.ParentName = parent.DeptName
	} else if !errors.Is(err, repository.ErrDeptNotFound) {
		return nil, err
	}

	_ = cache.Put(ctx, constant.CacheSysDept, deptCacheKey(deptID), out, constant.CacheTTLSysDept)
	return out, nil
}

// QueryList 按条件查部门列表（对应 Java selectDeptList）。
// 返回扁平列表而非树：前端拿到后自行 listToTree，与 Java 一致。
func (s *DeptService) QueryList(ctx context.Context, q bo.SysDeptQueryBo) ([]*vo.SysDeptVo, error) {
	rows, err := repository.NewDeptRepository(database.DB()).SelectList(ctx, q)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysDeptVoList(rows), nil
}

// QueryListExcludeChild 查部门列表并剔除 deptID 自身及其全部后代
// （对应 Java SysDeptController#excludeChild）。
//
// 在内存里过滤而非用 SQL 排除：Java 就是取全量后 removeIf，且这里要的是
// "祖先链包含 deptID"，与 SelectList 的排序/条件无关，多一个查询分支不划算。
func (s *DeptService) QueryListExcludeChild(ctx context.Context,
	deptID int64) ([]*vo.SysDeptVo, error) {

	rows, err := s.QueryList(ctx, bo.SysDeptQueryBo{})
	if err != nil {
		return nil, err
	}

	out := make([]*vo.SysDeptVo, 0, len(rows))
	for _, d := range rows {
		if d.DeptID == deptID || containsAncestor(d.Ancestors, deptID) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// SelectDeptTreeList 按条件查部门并组装成前端下拉树（对应 Java selectDeptTreeList
// + buildDeptTreeSelect）。
//
// 走 tree.BuildMultiRoot 而非 Build：按条件过滤后祖先可能不在结果集内，悬空父级要当根挂接，
// 与 Java TreeBuildUtils.buildMultiRoot 一致。disabled 按状态回填，前端据此灰掉停用部门。
func (s *DeptService) SelectDeptTreeList(ctx context.Context,
	q bo.SysDeptQueryBo) ([]*tree.Tree[int64], error) {

	rows, err := s.QueryList(ctx, q)
	if err != nil {
		return nil, err
	}
	return tree.BuildMultiRoot(rows, func(d *vo.SysDeptVo) int64 { return d.DeptID },
		func(d *vo.SysDeptVo) int64 { return d.ParentID },
		func(d *vo.SysDeptVo, node *tree.Tree[int64]) {
			node.ID = d.DeptID
			node.ParentID = d.ParentID
			node.Name = d.DeptName
			node.Weight = d.OrderNum
			node.SetExtra("disabled", d.Status == constant.StatusDisable)
		}), nil
}

// QueryByIDs 查启用状态的部门供选择框用（对应 Java selectDeptByIds）。
// ids 为空时返回全部启用部门，与 Java optionselect 不传参的语义一致。
func (s *DeptService) QueryByIDs(ctx context.Context, ids []int64) ([]*vo.SysDeptVo, error) {
	rows, err := repository.NewDeptRepository(database.DB()).SelectNormalByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysDeptVoList(rows), nil
}

// CheckDeptNameUnique 校验同一上级下的部门名称是否可用（对齐 Java 的「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *DeptService) CheckDeptNameUnique(ctx context.Context, deptName string,
	parentID, excludeID int64) (bool, error) {

	exists, err := repository.NewDeptRepository(database.DB()).
		ExistsByDeptName(ctx, deptName, parentID, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// CheckDeptDataScope 校验当前用户能否访问该部门（对应 Java checkDeptDataScope）。
//
// Java 靠 @DataPermission 给 countDeptById 注入部门隔离条件，Go 侧数据权限尚未落地，
// 此处等价于"部门存在即放行"——正是 Java 在无隔离条件时的行为。等数据权限落地后
// 只需给这次 count 挂上过滤，调用点不必改。
func (s *DeptService) CheckDeptDataScope(ctx context.Context, userID, deptID int64) error {
	if deptID <= 0 {
		return nil
	}
	if loginhelper.IsSuperAdmin(userID) {
		return nil
	}

	if _, err := repository.NewDeptRepository(database.DB()).SelectByID(ctx, deptID); err != nil {
		if errors.Is(err, repository.ErrDeptNotFound) {
			return errs.New(0, "没有权限访问部门数据！", "")
		}
		return err
	}
	return nil
}

// InsertDept 新增部门（对应 Java insertDept + @CacheEvict(SYS_DEPT_AND_CHILD, allEntries)）。
// 部门名称重复时返回 ErrDeptNameExists；插入成功后回填 b.DeptID。
func (s *DeptService) InsertDept(ctx context.Context, b *bo.SysDeptBo) error {
	if b == nil {
		return errors.New("service: 部门入参为空")
	}

	unique, err := s.CheckDeptNameUnique(ctx, b.DeptName, b.ParentID, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrDeptNameExists
	}

	repo := repository.NewDeptRepository(database.DB())
	parent, err := repo.SelectByID(ctx, b.ParentID)
	if err != nil {
		if errors.Is(err, repository.ErrDeptNotFound) {
			return errs.New(0, "父部门不存在", "")
		}
		return err
	}
	// 停用的部门下不允许挂新部门：否则新部门一出生就在一条断掉的启用链上。
	if parent.Status != constant.StatusNormal {
		return errs.New(0, "部门停用，不允许新增", "")
	}

	add := bo.Conv.ConvertToSysDept(b)
	add.DeptID = snowflake.Next() // dept_id 无 auto_increment
	add.Ancestors = parent.Ancestors + "," + strconv.FormatInt(parent.DeptID, 10)
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repo.Insert(ctx, add); err != nil {
		return err
	}
	b.DeptID = add.DeptID
	// 祖先链变了，按部门取子树的缓存整组失效。
	_ = cache.EvictGroup(ctx, constant.CacheSysDeptAndChild)
	return nil
}

// UpdateDept 修改部门（对应 Java updateDept + @CacheEvict(SYS_DEPT key=deptId, SYS_DEPT_AND_CHILD allEntries)）。
//
// userID 用于换父部门时的越权校验，对齐 Java 在 updateDept 内部再调一次 checkDeptDataScope。
func (s *DeptService) UpdateDept(ctx context.Context, userID int64, b *bo.SysDeptBo) error {
	if b == nil {
		return errors.New("service: 部门入参为空")
	}
	if b.DeptID <= 0 {
		return errors.New("service: 部门主键不能为空")
	}

	// 三道校验的先后顺序对齐 Java edit 的 if-else 链：名称重复 → 上级是自己 → 停用前置条件。
	// 同时触发多条时，前端看到的提示才与 Java 一致。
	unique, err := s.CheckDeptNameUnique(ctx, b.DeptName, b.ParentID, b.DeptID) // 排除自身
	if err != nil {
		return err
	}
	if !unique {
		return ErrDeptNameExists
	}
	// 认自己作父会让祖先链自指，构树时直接成环。
	if b.ParentID == b.DeptID {
		return ErrDeptParentIsSelf
	}

	repo := repository.NewDeptRepository(database.DB())
	old, err := repo.SelectByID(ctx, b.DeptID)
	if err != nil {
		if errors.Is(err, repository.ErrDeptNotFound) {
			return ErrDeptNotFound
		}
		return err
	}

	// 停用前必须确认名下没有还在用的部门/用户，否则会留下"启用的子部门挂在停用父级下"的断链。
	if b.Status == constant.StatusDisable {
		if err := s.checkDisableAllowed(ctx, b.DeptID); err != nil {
			return err
		}
	}

	ancestors := old.Ancestors
	if old.ParentID != b.ParentID {
		// 换父部门要单独校验新父级的访问权限，避免把部门挪进自己看不见的子树里越权。
		if err := s.CheckDeptDataScope(ctx, userID, b.ParentID); err != nil {
			return err
		}
		newParent, err := repo.SelectByID(ctx, b.ParentID)
		if err != nil {
			if errors.Is(err, repository.ErrDeptNotFound) {
				return errs.New(0, "父部门不存在", "")
			}
			return err
		}
		ancestors = newParent.Ancestors + "," + strconv.FormatInt(newParent.DeptID, 10)
		// 先改子孙再改自己：这里没有事务包裹，先落自己的话中途失败会留下
		// 父子祖先链互相矛盾的状态，而先改子孙失败则自己还没动，重试即可收敛。
		if err := s.updateChildrenAncestors(ctx, b.DeptID, old.Ancestors, ancestors); err != nil {
			return err
		}
	}

	columns := buildDeptUpdateColumns(b)
	columns["ancestors"] = ancestors
	if _, err := repo.UpdateByID(ctx, b.DeptID, columns); err != nil {
		return err
	}
	s.evictDept(ctx, b.DeptID)

	// 启用一个有上级的部门时，把整条祖先链一并启用——否则它挂在停用父级下，前端树上看不见。
	if b.Status == constant.StatusNormal && ancestors != "" &&
		ancestors != constant.RootDeptAncestors {
		if err := s.enableAncestors(ctx, ancestors); err != nil {
			return err
		}
	}
	return nil
}

// buildDeptUpdateColumns 组装修改部门的更新列。ancestors 由调用方按新旧父级另行补上。
func buildDeptUpdateColumns(b *bo.SysDeptBo) map[string]any {
	columns := map[string]any{
		"parent_id":     b.ParentID,
		"dept_name":     b.DeptName,
		"dept_category": b.DeptCategory,
		"order_num":     b.OrderNum,
		"leader":        b.Leader,
		// 一律写入，让前端能把联系方式清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"phone": b.Phone,
		"email": b.Email,
	}
	// 状态缺省即视为不改：漏传字段不该把线上的 '0' 刷成空串，
	// 那会让该部门既不算启用也不算停用。等效于 Java updateById 对 null 字段的跳过。
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// checkDisableAllowed 校验部门是否允许停用（对应 Java edit 里 DISABLE 分支的两道拦截）。
func (s *DeptService) checkDisableAllowed(ctx context.Context, deptID int64) error {
	count, err := repository.NewDeptRepository(database.DB()).CountNormalChildren(ctx, deptID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errs.New(0, "该部门包含未停用的子部门!", "")
	}

	exist, err := repository.NewUserRepository(database.DB()).ExistsByDeptID(ctx, deptID)
	if err != nil {
		return err
	}
	if exist {
		return errs.New(0, "该部门下存在已分配用户，不能禁用!", "")
	}
	return nil
}

// updateChildrenAncestors 把后代的祖先链前缀从 oldAncestors 换成 newAncestors
// （对应 Java updateDeptChildren）。
func (s *DeptService) updateChildrenAncestors(ctx context.Context, deptID int64,
	oldAncestors, newAncestors string) error {

	repo := repository.NewDeptRepository(database.DB())
	children, err := repo.SelectChildrenByAncestor(ctx, deptID)
	if err != nil {
		return err
	}

	for _, child := range children {
		// 只替换第一处（对齐 Java replaceOnce）：祖先链里同一段主键序列理论上不会重复，
		// 但全局替换一旦遇到重复段会把子树接到错误的位置上。
		updated := strings.Replace(child.Ancestors, oldAncestors, newAncestors, 1)
		if updated == child.Ancestors {
			continue
		}
		if _, err := repo.UpdateByID(ctx, child.DeptID,
			map[string]any{"ancestors": updated}); err != nil {
			return err
		}
		s.evictDept(ctx, child.DeptID)
	}
	return nil
}

// enableAncestors 把祖先链上的部门全部置为启用（对应 Java updateParentDeptStatusNormal）。
func (s *DeptService) enableAncestors(ctx context.Context, ancestors string) error {
	ids := parseAncestorIDs(ancestors)
	if len(ids) == 0 {
		return nil
	}

	if _, err := repository.NewDeptRepository(database.DB()).
		UpdateStatusNormalByIDs(ctx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		s.evictDept(ctx, id)
	}
	return nil
}

// DeleteDeptByID 删除部门（对应 Java deleteDeptById
// + @CacheEvict(SYS_DEPT key=deptId, SYS_DEPT_AND_CHILD allEntries)）。
//
// 四道前置拦截（默认部门 / 下级部门 / 用户 / 岗位）留在调用方 handler 之外、本方法之内，
// 与 Java 把它们摊在 Controller 的做法不同：这些是数据完整性约束而非 HTTP 关注点，
// 放这里才能挡住将来其它调用路径。
func (s *DeptService) DeleteDeptByID(ctx context.Context, userID, deptID int64) error {
	if deptID <= 0 {
		return errors.New("service: 部门主键不能为空")
	}
	// 默认部门是审计字段 create_dept 的兜底归属，删掉会让存量数据指向空部门。
	if deptID == constant.DefaultDeptID {
		return errs.New(response.CodeWarn, "默认部门,不允许删除", "")
	}

	deptRepo := repository.NewDeptRepository(database.DB())
	hasChild, err := deptRepo.ExistsByParentID(ctx, deptID)
	if err != nil {
		return err
	}
	if hasChild {
		return errs.New(response.CodeWarn, "存在下级部门,不允许删除", "")
	}

	existUser, err := repository.NewUserRepository(database.DB()).ExistsByDeptID(ctx, deptID)
	if err != nil {
		return err
	}
	if existUser {
		return errs.New(response.CodeWarn, "部门存在用户,不允许删除", "")
	}

	postCount, err := repository.NewPostRepository(database.DB()).CountByDeptID(ctx, deptID)
	if err != nil {
		return err
	}
	if postCount > 0 {
		return errs.New(response.CodeWarn, "部门存在岗位,不允许删除", "")
	}

	if err := s.CheckDeptDataScope(ctx, userID, deptID); err != nil {
		return err
	}

	affected, err := deptRepo.DeleteByID(ctx, deptID)
	if err != nil {
		return err
	}
	// 逻辑删除下 0 行只可能是主键不存在或已被删，对齐 Java toAjax(0) 的失败口径。
	if affected == 0 {
		return ErrDeptNotFound
	}
	// 删除后再失效：提前清缓存会让删除失败时白丢热数据。
	s.evictDept(ctx, deptID)
	return nil
}

// evictDept 失效单个部门缓存，并连带清空按部门取子树的缓存组。
// 后者是 allEntries 语义：子树结果按祖先链聚合，任一部门变动都可能影响别的 key。
func (s *DeptService) evictDept(ctx context.Context, deptID int64) {
	_ = cache.Evict(ctx, constant.CacheSysDept, deptCacheKey(deptID))
	_ = cache.EvictGroup(ctx, constant.CacheSysDeptAndChild)
}

// deptCacheKey 部门缓存的 key，与 Java @Cacheable(key = "#deptId") 同形。
func deptCacheKey(deptID int64) string {
	return strconv.FormatInt(deptID, 10)
}

// parseAncestorIDs 切分祖先链取出有效部门主键。
// 根部门的祖先链是 "0"，那不是真实部门，按主键必须为正过滤掉。
func parseAncestorIDs(ancestors string) []int64 {
	parts := strings.Split(ancestors, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// containsAncestor 判断祖先链是否包含指定部门，等价于 SQL 的 FIND_IN_SET。
// 按整段比对而非子串匹配：主键 100 不该命中祖先链里的 1001。
func containsAncestor(ancestors string, deptID int64) bool {
	want := strconv.FormatInt(deptID, 10)
	for _, p := range strings.Split(ancestors, ",") {
		if strings.TrimSpace(p) == want {
			return true
		}
	}
	return false
}
