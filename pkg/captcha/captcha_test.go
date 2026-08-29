package captcha

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"ruoyi-go-vue-plus/pkg/config"
)

// TestNewDisabled 未启用时应返回 no-op 实例。
func TestNewDisabled(t *testing.T) {
	c, err := New(config.CaptchaConfig{Enable: false})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	if c.enabled {
		t.Error("Enable=false 时 enabled 应为 false")
	}

	// 未启用时 Generate 不碰 Redis，可直接调用。
	vo, err := c.Generate(t.Context())
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if vo.CaptchaEnabled || vo.UUID != "" || vo.Img != "" {
		t.Errorf("未启用时应返回空 Vo, got %+v", vo)
	}

	// 未启用时 Validate 直接放行，任何值都不该报错。
	if err := c.Validate(t.Context(), "", "任意值"); err != nil {
		t.Errorf("未启用时 Validate 应放行, got %v", err)
	}
}

// TestNewUnknownType 未知类型应报错。
func TestNewUnknownType(t *testing.T) {
	if _, err := New(config.CaptchaConfig{Enable: true, Type: "qrcode"}); err == nil {
		t.Error("未知类型应返回错误")
	}
}

// TestNextMath 算术题的题面与答案必须自洽，且操作数受 numberLength 约束。
func TestNextMath(t *testing.T) {
	for _, numberLength := range []int{1, 2, 3} {
		bound := 1
		for range numberLength {
			bound *= 10
		}

		// 随机出题，多跑几轮覆盖三种运算符。
		for range 300 {
			question, answer := nextMath(numberLength)

			expr, ok := strings.CutSuffix(question, "=?")
			if !ok {
				t.Fatalf("题面应以 =? 结尾, got %q", question)
			}

			var a, b int
			var want int
			switch {
			case strings.Contains(expr, "+"):
				a, b = parseOperands(t, expr, "+")
				want = a + b
			case strings.Contains(expr, "-"):
				a, b = parseOperands(t, expr, "-")
				want = a - b
			case strings.Contains(expr, "*"):
				a, b = parseOperands(t, expr, "*")
				want = a * b
			default:
				t.Fatalf("无法识别运算符: %q", expr)
			}

			if got, _ := strconv.Atoi(answer); got != want {
				t.Errorf("题面 %q 答案应为 %d, got %s", question, want, answer)
			}
			// 对照 Java MathGenerator：结果非负。
			if want < 0 {
				t.Errorf("题面 %q 结果为负数 %d", question, want)
			}
			if a >= bound || b >= bound {
				t.Errorf("numberLength=%d 时操作数应小于 %d, got %d 和 %d",
					numberLength, bound, a, b)
			}
		}
	}
}

// parseOperands 从 "3+5" 形式的表达式里取两个操作数。
func parseOperands(t *testing.T, expr, op string) (int, int) {
	t.Helper()

	left, right, ok := strings.Cut(expr, op)
	if !ok {
		t.Fatalf("表达式 %q 不含 %q", expr, op)
	}
	a, err := strconv.Atoi(left)
	if err != nil {
		t.Fatalf("左操作数非法 %q: %v", left, err)
	}
	b, err := strconv.Atoi(right)
	if err != nil {
		t.Fatalf("右操作数非法 %q: %v", right, err)
	}
	return a, b
}

// TestStripB64Prefix 必须剥掉 base64Captcha 自带的 data URI 前缀，
// 否则前端再拼一次前缀会导致图裂。
func TestStripB64Prefix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"带 png 前缀", "data:image/png;base64,iVBORw0KGgo=", "iVBORw0KGgo="},
		{"无前缀原样返回", "iVBORw0KGgo=", "iVBORw0KGgo="},
		{"空串", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripB64Prefix(tc.in); got != tc.want {
				t.Errorf("stripB64Prefix(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestDrawCaptcha 两种类型都应能画出合法的 PNG，且输出为裸 base64。
func TestDrawCaptcha(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.CaptchaConfig
	}{
		{"math", config.CaptchaConfig{Enable: true, Type: config.CaptchaTypeMath, NumberLength: 1}},
		{"char", config.CaptchaConfig{Enable: true, Type: config.CaptchaTypeChar, CharLength: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(tc.cfg)
			if err != nil {
				t.Fatalf("New 失败: %v", err)
			}

			question, answer := c.next()
			if question == "" || answer == "" {
				t.Fatalf("题面/答案不应为空, got %q / %q", question, answer)
			}
			if tc.cfg.Type == config.CaptchaTypeChar {
				if question != answer {
					t.Errorf("字符验证码题面应等于答案, got %q / %q", question, answer)
				}
				if len([]rune(answer)) != tc.cfg.CharLength {
					t.Errorf("字符验证码长度应为 %d, got %q", tc.cfg.CharLength, answer)
				}
			}

			item, err := c.driver.DrawCaptcha(question)
			if err != nil {
				t.Fatalf("DrawCaptcha 失败: %v", err)
			}

			img := stripB64Prefix(item.EncodeB64string())
			if strings.HasPrefix(img, "data:") {
				t.Error("返回给前端的 img 不应含 data URI 前缀")
			}
			raw, err := base64.StdEncoding.DecodeString(img)
			if err != nil {
				t.Fatalf("img 不是合法 base64: %v", err)
			}
			// PNG 魔数。
			if !strings.HasPrefix(string(raw), "\x89PNG") {
				t.Error("解码结果不是 PNG")
			}
		})
	}
}

// TestKey 验证码键必须落在 global:captcha_codes: 命名空间下。
func TestKey(t *testing.T) {
	if got, want := key("abc123"), "global:captcha_codes:abc123"; got != want {
		t.Errorf("key() = %q, want %q", got, want)
	}
}

// TestExpiration 有效期应为 2 分钟，对照 Java Constants.CAPTCHA_EXPIRATION。
func TestExpiration(t *testing.T) {
	if expiration.Minutes() != 2 {
		t.Errorf("有效期应为 2 分钟, got %v", expiration)
	}
}
