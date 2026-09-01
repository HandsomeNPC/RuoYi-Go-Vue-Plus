package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/oplog"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// OperLogService 操作日志业务逻辑，对应 Java SysOperLogServiceImpl。
type OperLogService struct{}

// OperLogSvcApp 包级实例。
var OperLogSvcApp = new(OperLogService)

// RecordOper 记录操作日志，对应 Java SysOperLogServiceImpl.recordOper
// （@Async @EventListener）。签名满足 oplog.Recorder，由 main 注册给 pkg/oplog。
//
// **异步**：反查 IP 归属地要读地址库、落库要走网络，都不该占住请求线程；调用方
// 拿不到错误——操作日志写失败不该影响业务结果，与 Java @Async 语义一致。因此
// ctx 必须已脱离请求生命周期（pkg/oplog 侧传的是 context.WithoutCancel）。
func (s *OperLogService) RecordOper(ctx context.Context, evt *oplog.Event) {
	if evt == nil {
		return
	}
	go func() {
		// 后台 goroutine 里 panic 会直接带走整个进程，必须自己兜住。
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[system] 记录操作日志 panic: %v", r)
			}
		}()
		if err := s.recordOper(ctx, evt); err != nil {
			log.Printf("[system] 记录操作日志失败: %v", err)
		}
	}()
}

// recordOper 同步执行记录操作日志的实际逻辑，供 RecordOper 在 goroutine 里调用。
func (s *OperLogService) recordOper(ctx context.Context, evt *oplog.Event) error {
	b := &bo.SysOperLogBo{
		Title:         evt.Title,
		BusinessType:  evt.BusinessType,
		Method:        evt.Method,
		RequestMethod: evt.RequestMethod,
		OperatorType:  evt.OperatorType,
		OperName:      evt.OperName,
		UserID:        evt.UserID,
		DeptID:        evt.DeptID,
		DeptName:      evt.DeptName,
		ClientKey:     evt.ClientKey,
		DeviceType:    evt.DeviceType,
		Browser:       evt.Browser,
		OS:            evt.OS,
		OperURL:       evt.OperURL,
		OperIP:        evt.OperIP,
		// 对照 Java recordOper 里的 AddressUtils.getRealAddressByIP：
		// 归属地在消费侧补，注解侧只带原始 IP。
		OperLocation: ip.RealAddressByIP(evt.OperIP),
		OperParam:    evt.OperParam,
		JSONResult:   evt.JSONResult,
		Status:       evt.Status,
		ErrorMsg:     evt.ErrorMsg,
		CostTime:     evt.CostTime,
	}
	return s.InsertOperLog(ctx, b)
}

// InsertOperLog 新增操作日志，对应 Java insertOperlog。
// 落库时间取当前时刻，主键由雪花发号（oper_id 无 auto_increment）。
func (s *OperLogService) InsertOperLog(ctx context.Context, b *bo.SysOperLogBo) error {
	if b == nil {
		return fmt.Errorf("service: 操作日志为空")
	}
	l := bo.Conv.ConvertToSysOperLog(b)
	now := time.Now()
	l.OperTime = &now
	l.OperID = snowflake.Next()
	return repository.NewOperLogRepository(database.DB()).Insert(ctx, l)
}
