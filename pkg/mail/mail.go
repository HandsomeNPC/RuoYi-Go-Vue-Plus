// Package mail 邮件发送。
//
// 只做「纯文本、单收件人」——本项目发邮件的唯一场景是登录验证码，
// 附件/HTML/群发都用不上，引一个第三方库不划算，标准库 net/smtp 足够。
//
// 初始化对照 captcha.Init：mail.Init() 无参，自读 config.Get().Mail 设包级实例，
// 业务侧用包级 mail.Send() / mail.Enabled()。
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"ruoyi-go-vue-plus/pkg/config"
)

// ErrDisabled 邮件功能未开启。由调用方翻译成给前端看的文案。
var ErrDisabled = errors.New("mail: 邮件功能未开启")

// dialTimeout 建连超时。SMTP 服务器不可达时若不设超时，
// 请求会一直挂到客户端断开，把 handler 协程占死。
const dialTimeout = 10 * time.Second

// Mailer 邮件发送器。
type Mailer struct {
	cfg config.MailConfig
}

// New 按配置构造 Mailer。!Enabled 时返回 no-op（Send 直接返回 ErrDisabled）。
func New(cfg config.MailConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// 包级默认实例（对照 captcha.defaultCaptcha）。
var (
	mu            sync.RWMutex
	defaultMailer *Mailer
)

// Init 按 config.Get().Mail 构造并设包级默认实例。须在 config.Load 之后调用。
func Init() {
	c := config.Get()
	mu.Lock()
	defaultMailer = New(c.Mail)
	mu.Unlock()
	log.Printf("[%s] mail 已就绪: enabled=%t host=%s:%d",
		c.Server.Name, c.Mail.Enabled, c.Mail.Host, c.Mail.Port)
}

// get 返回包级默认实例，未调用 Init 会 panic。
func get() *Mailer {
	mu.RLock()
	m := defaultMailer
	mu.RUnlock()
	if m == nil {
		panic("mail: 尚未初始化，请先调用 mail.Init")
	}
	return m
}

// Enabled 邮件功能是否开启。
func Enabled() bool { return get().cfg.Enabled }

// Send 发一封纯文本邮件，未开启时返回 ErrDisabled。
func Send(ctx context.Context, to, subject, body string) error {
	return get().Send(ctx, to, subject, body)
}

// Send 见包级 Send。
//
// ctx 目前只用于取消建连（net.Dialer），SMTP 会话本身不可中途取消——
// 一封验证码邮件的会话很短，不值得为此自己实现协议层超时。
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	if !m.cfg.Enabled {
		return ErrDisabled
	}
	if to == "" {
		return errors.New("mail: 收件人为空")
	}

	client, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mail: 认证失败: %w", err)
	}
	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mail: 设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mail: 设置收件人失败: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: 打开数据通道失败: %w", err)
	}
	if _, err := w.Write([]byte(m.message(to, subject, body))); err != nil {
		return fmt.Errorf("mail: 写入邮件内容失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: 提交邮件失败: %w", err)
	}
	return client.Quit()
}

// dial 建立 SMTP 连接。
//
// 两种加密方式不能混：SSL 是连上来就握手（465），STARTTLS 是先明文连、
// 再用 STARTTLS 命令升级（587）。对着 465 发 STARTTLS 会直接卡住。
func (m *Mailer) dial(ctx context.Context) (*smtp.Client, error) {
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprint(m.cfg.Port))
	dialer := &net.Dialer{Timeout: dialTimeout}

	if m.cfg.SSL {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
		if err != nil {
			return nil, fmt.Errorf("mail: 连接 %s 失败: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("mail: 建立 SMTP 会话失败: %w", err)
		}
		return client, nil
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mail: 连接 %s 失败: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mail: 建立 SMTP 会话失败: %w", err)
	}
	if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("mail: STARTTLS 升级失败: %w", err)
	}
	return client, nil
}

// message 拼 RFC 5322 报文。
func (m *Mailer) message(to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + m.cfg.From + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	// 主题必须按 RFC 2047 编码：中文主题直接塞进报文头，多数客户端显示成乱码。
	sb.WriteString("Subject: " + encodeHeader(subject) + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	// 空行分隔头与正文，缺了整封信会被当成头解析。
	sb.WriteString("\r\n")
	sb.WriteString(normalizeNewlines(body))
	return sb.String()
}

// encodeHeader 按 RFC 2047 编码报文头里的非 ASCII 内容。
// 纯 ASCII 原样返回（BEncoding 对 ASCII 也不做转换）。
func encodeHeader(s string) string {
	return mime.BEncoding.Encode("UTF-8", s)
}

// normalizeNewlines 把裸 \n 统一成 \r\n。
// SMTP 以 \r\n 为行分隔符，混用会让部分服务器把正文截断。
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
