package dictdata

import (
	"context"
	"errors"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/cache"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/constant"
	pkgrepo "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repository"
	"github.com/HandsomeNPC/ruoyi-common-go/snowflake"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	dictdatarepository "ruoyi-go-vue-plus/internal/system/repository/dictdata"
)

var ErrDictDataNotFound = errors.New("service: 字典数据不存在")

// ErrDictDataValueExists 同一字典类型下的键值已被占用。
var ErrDictDataValueExists = errors.New("service: 字典键值已存在")

// QueryPageList 按条件分页查字典数据。
func QueryPageList(ctx context.Context, q bo.SysDictDataQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysDictDataVo], error) {

	res, err := dictdatarepository.SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysDictDataVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysDictDataVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查字典数据，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func QueryList(ctx context.Context, q bo.SysDictDataQueryBo,
	limit int) ([]*vo.SysDictDataVo, error) {

	rows, err := dictdatarepository.SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysDictDataVoList(rows), nil
}

// QueryByID 按字典编码查字典数据，
// 不存在时返回 ErrDictDataNotFound。
func QueryByID(ctx context.Context, dictCode int64) (*vo.SysDictDataVo, error) {
	data, err := dictdatarepository.SelectByID(ctx, dictCode)
	if err != nil {
		if errors.Is(err, dictdatarepository.ErrDictDataNotFound) {
			return nil, ErrDictDataNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysDictDataVo(data), nil
}

// CheckDictDataUnique 校验同一字典类型下的键值是否可用（唯一即 true）。
// excludeID > 0 时排除该主键，供修改场景复用。
func CheckDictDataUnique(ctx context.Context, dictType, dictValue string,
	excludeID int64) (bool, error) {

	exists, err := dictdatarepository.
		ExistsByTypeAndValue(ctx, dictType, dictValue, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertDictData 新增字典数据。
// 同类型下键值重复时返回 ErrDictDataValueExists；插入成功后回填 b.DictCode。
func InsertDictData(ctx context.Context, b *bo.SysDictDataBo) error {
	if b == nil {
		return errors.New("service: 字典数据入参为空")
	}

	unique, err := CheckDictDataUnique(ctx, b.DictType, b.DictValue, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrDictDataValueExists
	}

	add := bo.Conv.ConvertToSysDictData(b)
	add.DictCode = snowflake.Next() // dict_code 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := dictdatarepository.Insert(ctx, add); err != nil {
		return err
	}
	b.DictCode = add.DictCode
	refreshTypeCache(ctx, add.DictType)
	return nil
}

// UpdateDictData 按字典编码修改字典数据。
// 同类型下键值被别的数据占用时返回 ErrDictDataValueExists；主键不存在返回 ErrDictDataNotFound。
func UpdateDictData(ctx context.Context, b *bo.SysDictDataBo) error {
	if b == nil {
		return errors.New("service: 字典数据入参为空")
	}
	if b.DictCode <= 0 {
		return errors.New("service: 字典数据主键不能为空")
	}

	unique, err := CheckDictDataUnique(ctx, b.DictType, b.DictValue, b.DictCode) // 排除自身
	if err != nil {
		return err
	}
	if !unique {
		return ErrDictDataValueExists
	}

	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"字典数据不存在"。
	old, err := dictdatarepository.SelectByID(ctx, b.DictCode)
	if err != nil {
		if errors.Is(err, dictdatarepository.ErrDictDataNotFound) {
			return ErrDictDataNotFound
		}
		return err
	}

	if _, err := dictdatarepository.UpdateByID(ctx, b.DictCode, buildDictDataUpdateColumns(b)); err != nil {
		return err
	}

	// 把这条数据挪到别的类型下时，原类型的缓存列表里还留着它，需一并刷新。
	if old.DictType != b.DictType {
		refreshTypeCache(ctx, old.DictType)
	}
	refreshTypeCache(ctx, b.DictType)
	return nil
}

// buildDictDataUpdateColumns 组装修改字典数据的更新列。
func buildDictDataUpdateColumns(b *bo.SysDictDataBo) map[string]any {
	columns := map[string]any{
		"dict_sort":  b.DictSort,
		"dict_label": b.DictLabel,
		"dict_value": b.DictValue,
		"dict_type":  b.DictType,
		// 三者一律写入，让前端能把样式与备注清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"css_class":  b.CssClass,
		"list_class": b.ListClass,
		"remark":     b.Remark,
	}
	// 是否默认缺省即视为不改：前端编辑表单根本不含该字段，一律写会把线上的 'Y'
	// 刷成空串，让 char(1) 既不是 'Y' 也不是 'N'。
	if b.IsDefault != "" {
		columns["is_default"] = b.IsDefault
	}
	return columns
}

// DeleteDictDataByIDs 批量删除字典数据。
func DeleteDictDataByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 字典数据主键不能为空")
	}

	// 先取回待删行，删除后才知道该刷哪些类型的缓存。
	rows, err := dictdatarepository.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	if _, err := dictdatarepository.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 删除后再刷新：提前清缓存会让删除失败时白丢一批热数据。
	// 同一类型可能命中多行，去重避免重复查库。
	seen := make(map[string]struct{}, len(rows))
	for _, data := range rows {
		if _, dup := seen[data.DictType]; dup {
			continue
		}
		seen[data.DictType] = struct{}{}
		refreshTypeCache(ctx, data.DictType)
	}
	return nil
}

// refreshTypeCache 重查该类型下的字典数据并回写 SYS_DICT 缓存。
//
// 回写而非单纯失效：字典是全站高频读，删除类操作后紧接着的读请求不该都落到库上。
// 查库失败则退化为失效——宁可下次读穿，也不能留着与库不一致的旧列表。
func refreshTypeCache(ctx context.Context, dictType string) {
	if dictType == "" {
		return
	}
	list, err := selectDictDataByTypeUncached(ctx, dictType)
	if err != nil {
		_ = cache.Evict(ctx, constant.CacheSysDict, dictType)
		return
	}
	_ = cache.Put(ctx, constant.CacheSysDict, dictType, list, constant.CacheTTLSysDict)
}

// selectDictDataByTypeUncached 直接查库取某类型下的字典数据，恒返回非 nil 切片
// （同时保证 JSON 序列化成 [] 而非 null）。
//
// 与 dicttype 包里的同名函数体一致：两边指的是同一份 SYS_DICT 缓存，
// 拆包后各自保留一份以便互不依赖。
func selectDictDataByTypeUncached(ctx context.Context, dictType string) ([]*vo.SysDictDataVo, error) {
	rows, err := dictdatarepository.SelectByType(ctx, dictType)
	if err != nil {
		return nil, err
	}
	list := vo.Conv.ConvertToSysDictDataVoList(rows)
	if list == nil {
		return []*vo.SysDictDataVo{}, nil
	}
	return list, nil
}
