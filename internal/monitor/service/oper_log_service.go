// Package service 操作日志监控业务逻辑。
package service

import (
	"context"
	"fmt"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	systemrepo "ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// OperLogSvc 操作日志监控服务，对应 Java SysOperLogServiceImpl 的查询/删除部分
// （记录落库 part 已在 internal/system/service 的 OperLogSvcApp，这里只做监控侧）。
type OperLogSvc struct{}

// OperLogSvcApp 包级实例。
var OperLogSvcApp = new(OperLogSvc)

// QueryPageList 按条件分页查操作日志。
func (s *OperLogSvc) QueryPageList(ctx context.Context, q bo.SysOperLogQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysOperLogVo], error) {

	res, err := systemrepo.NewOperLogRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysOperLogVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysOperLogVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查操作日志，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *OperLogSvc) QueryList(ctx context.Context, q bo.SysOperLogQueryBo,
	limit int) ([]*vo.SysOperLogVo, error) {

	rows, err := systemrepo.NewOperLogRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysOperLogVoList(rows), nil
}

// DeleteByIDs 批量删除操作日志。未命中的主键静默跳过（对齐 Java deleteOperLogByIds 不做存在性预校验）。
func (s *OperLogSvc) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("service: 操作日志主键不能为空")
	}
	if _, err := systemrepo.NewOperLogRepository(database.DB()).DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	return nil
}

// Clean 清空全部操作日志（对应 Java cleanOperLog）。
//
// 未挂分布式锁（Java 侧 cleanOperLog 带 @Lock4j）：clean 是全表 delete，幂等——
// 并发两次只是第二次受影响 0 行，结果一致，故省去锁。本项目亦无 Lock4j 等价物。
func (s *OperLogSvc) Clean(ctx context.Context) error {
	return systemrepo.NewOperLogRepository(database.DB()).Clean(ctx)
}
