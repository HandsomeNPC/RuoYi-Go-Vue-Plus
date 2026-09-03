package mail

import (
	"mime"
	"strings"
	"testing"

	"ruoyi-go-vue-plus/pkg/config"
)

// testMailer 构造一个启用状态的发送器，仅用于验证报文拼装（不连 SMTP）。
func testMailer() *Mailer {
	return New(config.MailConfig{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    465,
		From:    "noreply@example.com",
		User:    "noreply@example.com",
		SSL:     true,
	})
}

// TestEncodeHeaderNonASCII 中文主题须按 RFC 2047 编码，且能原样解回。
// 不编码的话多数邮件客户端把主题显示成乱码——这是手写 SMTP 最易漏的一处。
func TestEncodeHeaderNonASCII(t *testing.T) {
	const subject = "登录验证码"
	got := encodeHeader(subject)

	if !strings.HasPrefix(got, "=?UTF-8?b?") && !strings.HasPrefix(got, "=?UTF-8?B?") {
		t.Errorf("中文主题未被编码: %q", got)
	}
	// 编码结果必须可逆，否则收件人看到的仍是乱码。
	decoded, err := new(mime.WordDecoder).DecodeHeader(got)
	if err != nil {
		t.Fatalf("解码 %q 失败: %v", got, err)
	}
	if decoded != subject {
		t.Errorf("解码得到 %q, want %q", decoded, subject)
	}
}

// TestEncodeHeaderASCII 纯 ASCII 主题不该被套上编码壳，否则平白增大报文。
func TestEncodeHeaderASCII(t *testing.T) {
	const subject = "Login Code"
	if got := encodeHeader(subject); got != subject {
		t.Errorf("encodeHeader(%q) = %q，纯 ASCII 不应编码", subject, got)
	}
}

// TestMessageStructure 报文头齐全、且头与正文之间有且仅有一个空行。
// 少了那个空行，整封信会被当成报文头解析，正文丢失。
func TestMessageStructure(t *testing.T) {
	m := testMailer()
	msg := m.message("user@example.com", "登录验证码", "您本次验证码为：1234")

	for _, want := range []string{
		"From: noreply@example.com\r\n",
		"To: user@example.com\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("报文缺少 %q\n实际:\n%s", want, msg)
		}
	}

	head, body, ok := strings.Cut(msg, "\r\n\r\n")
	if !ok {
		t.Fatalf("报文头与正文之间缺少空行\n实际:\n%s", msg)
	}
	if strings.Contains(head, "验证码为") {
		t.Error("正文串进了报文头")
	}
	if !strings.Contains(body, "您本次验证码为：1234") {
		t.Errorf("正文内容不对: %q", body)
	}
}

// TestMessageNormalizesNewlines 正文里的裸 \n 统一成 \r\n。
// SMTP 以 \r\n 分行，混用会让部分服务器把正文截断。
func TestMessageNormalizesNewlines(t *testing.T) {
	m := testMailer()
	msg := m.message("user@example.com", "s", "第一行\n第二行\r\n第三行")

	_, body, _ := strings.Cut(msg, "\r\n\r\n")
	if strings.Contains(strings.ReplaceAll(body, "\r\n", ""), "\n") {
		t.Errorf("正文仍含裸 \\n: %q", body)
	}
	// 已是 \r\n 的不该被重复转成 \r\r\n。
	if strings.Contains(body, "\r\r\n") {
		t.Errorf("正文出现重复回车: %q", body)
	}
	if want := "第一行\r\n第二行\r\n第三行"; body != want {
		t.Errorf("正文 = %q, want %q", body, want)
	}
}

// TestSendDisabled 未开启时返回 ErrDisabled，由 handler 翻译文案，不真去连 SMTP。
func TestSendDisabled(t *testing.T) {
	m := New(config.MailConfig{Enabled: false})
	if err := m.Send(t.Context(), "user@example.com", "s", "b"); err != ErrDisabled {
		t.Errorf("Send 返回 %v, want ErrDisabled", err)
	}
}

// TestSendEmptyRecipient 收件人为空直接拒绝，不必等 SMTP 服务器报错。
func TestSendEmptyRecipient(t *testing.T) {
	if err := testMailer().Send(t.Context(), "", "s", "b"); err == nil {
		t.Error("收件人为空时应报错")
	}
}
