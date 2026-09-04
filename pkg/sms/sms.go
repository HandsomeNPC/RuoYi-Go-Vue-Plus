// Package sms 短信发送。
//
// sms4j 聚合多厂商，Go 生态无对应物，故这里自留 Sender 接口，
// 内置阿里云一家实现；换/加厂商只需再写一个 Sender，调用方不动。
//
// 初始化对照 mail.Init / captcha.Init：sms.Init() 无参，自读 config.Get().SMS。
package sms

import (
	"context"
	"errors"
	"log"
	"sync"

	"ruoyi-go-vue-plus/pkg/config"
)

// ErrDisabled 短信功能未开启。由调用方翻译成给前端看的文案。
var ErrDisabled = errors.New("sms: 短信功能未开启")

// templateParamCode 模板变量名固定为 "code"，阿里云模板里写 ${code}，此处的键须与之一致。
const templateParamCode = "code"

// Sender 短信发送器。params 是模板变量表。
type Sender interface {
	Send(ctx context.Context, phone string, params map[string]string) error
}

// 包级默认实例。未开启短信时为 nil，故取用前一律判空。
var (
	mu            sync.RWMutex
	defaultSender Sender
)

// Init 按 config.Get().SMS 构造并设包级默认发送器。须在 config.Load 之后调用。
func Init() {
	c := config.Get()
	mu.Lock()
	if c.SMS.Enabled {
		defaultSender = newAliyunSender(c.SMS)
	} else {
		defaultSender = nil
	}
	mu.Unlock()
	log.Printf("[%s] sms 已就绪: enabled=%t", c.Server.Name, c.SMS.Enabled)
}

// SetSender 替换包级发送器，仅供测试注入桩件用。
func SetSender(s Sender) (restore func()) {
	mu.Lock()
	old := defaultSender
	defaultSender = s
	mu.Unlock()
	return func() {
		mu.Lock()
		defaultSender = old
		mu.Unlock()
	}
}

// sender 返回包级发送器，未开启时返回 nil。
func sender() Sender {
	mu.RLock()
	defer mu.RUnlock()
	return defaultSender
}

// Enabled 短信功能是否开启。
func Enabled() bool { return sender() != nil }

// SendCode 发一条验证码短信，未开启时返回 ErrDisabled。
func SendCode(ctx context.Context, phone, code string) error {
	s := sender()
	if s == nil {
		return ErrDisabled
	}
	return s.Send(ctx, phone, map[string]string{templateParamCode: code})
}
