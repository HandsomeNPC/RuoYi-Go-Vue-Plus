package service

import (
	"context"
	"log"
	"time"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/ip"
	"ruoyi-go-vue-plus/pkg/snowflake"
	"ruoyi-go-vue-plus/pkg/useragent"
)

// LoginInfoService 系统访问记录业务逻辑，对应 Java SysLoginInfoServiceImpl。
type LoginInfoService struct{}

// LoginInfoSvcApp 包级实例。
var LoginInfoSvcApp = new(LoginInfoService)

// RecordLoginInfo 记录登录信息，对应 Java SysLoginInfoServiceImpl.recordLoginInfo
// （@Async @EventListener）。
//
// **异步**：解析 UA、反查客户端、IP 归属地、落库全在后台 goroutine 里做，调用方不等待、
// 拿不到错误——登录成败不应受日志写入影响，这与 Java @Async 的语义一致。因此 ctx 必须
// 是脱离请求生命周期的（调用方传 context.WithoutCancel(...)），否则请求一结束
// ctx 就被取消，落库会失败。
func (s *LoginInfoService) RecordLoginInfo(ctx context.Context, evt *dto.LoginInfoEvent) {
	if evt == nil {
		return
	}
	go func() {
		// 后台 goroutine 里 panic 会直接带走整个进程，必须自己兜住。
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[system] 记录登录日志 panic: %v", r)
			}
		}()
		if err := s.recordLoginInfo(ctx, evt); err != nil {
			log.Printf("[system] 记录登录日志失败: %v", err)
		}
	}()
}

// recordLoginInfo 同步执行记录登录信息的实际逻辑，供 RecordLoginInfo 在 goroutine 里调用。
func (s *LoginInfoService) recordLoginInfo(ctx context.Context, evt *dto.LoginInfoEvent) error {
	b := &bo.SysLoginInfoBo{
		UserName:      evt.Username,
		IPAddr:        evt.IP,
		LoginLocation: ip.RealAddressByIP(evt.IP),
		Msg:           evt.Message,
		Status:        mapLoginStatus(evt.Status),
	}
	b.Browser, b.OS = useragent.Parse(evt.UserAgent)

	// 客户端信息：查不到不算失败，日志照记（对照 Java client 为 null 时跳过赋值）。
	if evt.ClientID != "" {
		client, err := ClientSvcApp.QueryByClientID(ctx, evt.ClientID)
		if err != nil {
			log.Printf("[system] 登录日志反查客户端 %s 失败: %v", evt.ClientID, err)
		} else {
			b.ClientKey = client.ClientKey
			b.DeviceType = client.DeviceType
		}
	}

	// 对照 Java log.info(getBlock(ip) + address + ...) 的方括号格式。
	log.Printf("[system] 登录日志 [%s][%s][%s][%s][%s]",
		b.IPAddr, b.LoginLocation, b.UserName, evt.Status, b.Msg)

	return s.InsertLoginInfo(ctx, b)
}

// InsertLoginInfo 新增系统登录日志，对应 Java insertLoginInfo。
// 落库时间取当前时刻，主键由雪花发号（info_id 无 auto_increment）。
func (s *LoginInfoService) InsertLoginInfo(ctx context.Context, b *bo.SysLoginInfoBo) error {
	info := bo.Conv.ConvertToSysLoginInfo(b)
	now := time.Now()
	info.LoginTime = &now
	info.InfoID = snowflake.Next()
	return repository.NewLoginInfoRepository(database.DB()).Insert(ctx, info)
}

// mapLoginStatus 把事件里的操作类型映射成 sys_login_info.status 的落表值，
// 对照 Java：LOGIN_SUCCESS / LOGOUT / REGISTER → SUCCESS("0")，LOGIN_FAIL → FAIL("1")。
// 未识别的取值返回空串，与 Java 两个分支都不命中时 status 留 null 的行为一致。
func mapLoginStatus(status string) string {
	switch status {
	case constant.ConstantLoginSuccess, constant.ConstantLogout, constant.ConstantRegister:
		return constant.ConstantSuccess
	case constant.ConstantLoginFail:
		return constant.ConstantFail
	default:
		return ""
	}
}
