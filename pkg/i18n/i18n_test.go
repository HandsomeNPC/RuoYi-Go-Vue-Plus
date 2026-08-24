package i18n

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// javaI18nDir 原项目词条目录，用于交叉验证。路径不存在时相关用例 Skip。
const javaI18nDir = `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus\ruoyi-admin\src\main\resources\i18n`

// rePositionalPlaceholder 匹配位置占位符 {0} {1}。
var rePositionalPlaceholder = regexp.MustCompile(`\{[0-9]+\}`)

// loadProperties 读一份 .properties，返回键值对。
func loadProperties(t *testing.T, path string) map[string]string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Skipf("读不到原项目词条 %s（跳过交叉验证）: %v", path, err)
	}
	defer f.Close()

	out := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		out[strings.TrimSpace(k)] = v
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return out
}

// TestCatalogsMatchJavaProperties 把 Go 侧词条与原项目 .properties 交叉验证。
func TestCatalogsMatchJavaProperties(t *testing.T) {
	cases := []struct {
		file    string
		catalog map[string]string
	}{
		// 中文表同时对应 messages.properties 与 messages_zh_CN.properties。
		{"messages_zh_CN.properties", messagesZhCN},
		{"messages_en_US.properties", messagesEnUS},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			want := loadProperties(t, filepath.Join(javaI18nDir, tc.file))

			for key, javaVal := range want {
				goVal, ok := tc.catalog[key]
				if !ok {
					t.Errorf("键 %q 在原项目里有、Go 侧缺失", key)
					continue
				}
				// 把 Java 的 {0}/{1} 折算成 %v 再比。
				expect := rePositionalPlaceholder.ReplaceAllString(javaVal, "%v")
				if goVal != expect {
					t.Errorf("键 %q 文案不一致:\n  Go   = %q\n  Java = %q（占位符折算后 %q）",
						key, goVal, javaVal, expect)
				}
			}

			for key := range tc.catalog {
				if _, ok := want[key]; !ok {
					t.Errorf("键 %q 是 Go 侧凭空多出来的，原项目里没有", key)
				}
			}
		})
	}
}

// TestCatalogKeySetsMatch 验证两份词条键集一致。
func TestCatalogKeySetsMatch(t *testing.T) {
	for key := range messagesZhCN {
		if _, ok := messagesEnUS[key]; !ok {
			t.Errorf("键 %q 只有中文词条，缺英文", key)
		}
	}
	for key := range messagesEnUS {
		if _, ok := messagesZhCN[key]; !ok {
			t.Errorf("键 %q 只有英文词条，缺中文", key)
		}
	}
}

// TestPlaceholderCountsMatchAcrossLocales 验证占位符个数跨语言一致。
func TestPlaceholderCountsMatchAcrossLocales(t *testing.T) {
	for key, zh := range messagesZhCN {
		en, ok := messagesEnUS[key]
		if !ok {
			continue
		}
		if zhN, enN := strings.Count(zh, "%v"), strings.Count(en, "%v"); zhN != enN {
			t.Errorf("键 %q 占位符个数不一致: 中文 %d 个, 英文 %d 个", key, zhN, enN)
		}
	}
}

// TestOnlySafeVerbsAndNamedPlaceholdersPreserved 验证只用 %v 且 {min}/{max} 保留。
func TestOnlySafeVerbsAndNamedPlaceholdersPreserved(t *testing.T) {
	reVerb := regexp.MustCompile(`%[a-zA-Z]`)

	for name, catalog := range map[string]map[string]string{
		"zh-CN": messagesZhCN,
		"en-US": messagesEnUS,
	} {
		for key, tmpl := range catalog {
			for _, verb := range reVerb.FindAllString(strings.ReplaceAll(tmpl, "%%", ""), -1) {
				if verb != "%v" {
					t.Errorf("[%s] 键 %q 含非 %%v 动词 %s: %q（迁移约定只用 %%v）",
						name, key, verb, tmpl)
				}
			}
		}
	}

	// {min}/{max} 必须还在。
	for _, key := range []string{
		"length.not.valid", "user.username.length.valid", "user.password.length.valid",
	} {
		for name, catalog := range map[string]map[string]string{
			"zh-CN": messagesZhCN,
			"en-US": messagesEnUS,
		} {
			tmpl := catalog[key]
			if !strings.Contains(tmpl, "{min}") || !strings.Contains(tmpl, "{max}") {
				t.Errorf("[%s] 键 %q 应保留 {min}/{max} 属性占位符, 实际 %q", name, key, tmpl)
			}
		}
	}
}

// TestParse 验证 Parse 的归一化与白名单校验。
func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   Locale
		wantOK bool
	}{
		{"下划线归一成连字符", "zh_CN", LocaleZhCN, true},
		{"大写归一成小写", "ZH-CN", LocaleZhCN, true},
		{"混合大小写加下划线", "En_us", LocaleEnUS, true},
		{"已是规范形态", "en-us", LocaleEnUS, true},
		{"仅语言不带地区", "en", "en", true},
		{"含脚本子标签", "zh-Hans-CN", "zh-hans-cn", true},
		{"两侧空白被裁掉", "  zh-CN  ", LocaleZhCN, true},
		{"列表形态取第一段", "en-US, zh-CN", LocaleEnUS, true},
		{"空串", "", "", false},
		{"仅空白", "   ", "", false},
		{"含换行(日志注入)", "zh-CN\nX-Injected: 1", "", false},
		{"含回车", "zh-CN\r", "", false},
		{"含分号", "zh-CN;q=0.9", "", false},
		{"含空格", "zh CN", "", false},
		{"含中文", "中文", "", false},
		{"超长", strings.Repeat("a", localeMaxLength+1), "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Parse(tc.in)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("Parse(%q) = (%q, %v), 期望 (%q, %v)",
					tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestParseRejectsRatherThanTruncates 验证超长输入整体拒绝而非截断。
func TestParseRejectsRatherThanTruncates(t *testing.T) {
	long := "zh-CN" + strings.Repeat("x", localeMaxLength)
	if got, ok := Parse(long); ok {
		t.Errorf("Parse(超长值) = (%q, true), 应整体拒绝而非截断", got)
	}
}

// TestContextRoundTrip 验证 context 存取往返。
func TestContextRoundTrip(t *testing.T) {
	ctx := NewContext(context.Background(), LocaleEnUS)
	if got := FromContext(ctx); got != LocaleEnUS {
		t.Errorf("FromContext = %q, 期望 %q", got, LocaleEnUS)
	}
}

// TestFromContextFallsBackToDefault 验证无语言或 nil context 回落默认。
func TestFromContextFallsBackToDefault(t *testing.T) {
	if got := FromContext(context.Background()); got != DefaultLocale {
		t.Errorf("未设置语言时 FromContext = %q, 期望 %q", got, DefaultLocale)
	}
	//lint:ignore SA1012 有意传 nil。
	if got := FromContext(nil); got != DefaultLocale { //nolint:staticcheck
		t.Errorf("nil context 时 FromContext = %q, 期望 %q", got, DefaultLocale)
	}
}

// TestMsgRendersPerLocale 验证 Msg 按 context 语言渲染。
func TestMsgRendersPerLocale(t *testing.T) {
	cases := []struct {
		loc  Locale
		want string
	}{
		{LocaleZhCN, "密码输入错误5次，账户锁定10分钟"},
		{LocaleEnUS, "Password input error 5 times, account locked for 10 minutes"},
	}
	for _, tc := range cases {
		t.Run(string(tc.loc), func(t *testing.T) {
			ctx := NewContext(context.Background(), tc.loc)
			got := Msg(ctx, "user.password.retry.limit.exceed", 5, 10)
			if got != tc.want {
				t.Errorf("Msg = %q, 期望 %q", got, tc.want)
			}
		})
	}
}

// TestMsgReturnsCodeWhenMissing 验证词条缺失时返回 code 本身。
func TestMsgReturnsCodeWhenMissing(t *testing.T) {
	const missing = "no.such.key.exists"
	if got := Msg(context.Background(), missing); got != missing {
		t.Errorf("Msg(缺失键) = %q, 期望原样返回 %q", got, missing)
	}
}

// TestMsgEmptyCode 验证空 code 不 panic 且原样返回。
func TestMsgEmptyCode(t *testing.T) {
	if got := Msg(context.Background(), ""); got != "" {
		t.Errorf("Msg(\"\") = %q, 期望空串", got)
	}
}

// TestMsgWithoutArgsKeepsTemplate 验证无参时返回原始模板。
func TestMsgWithoutArgsKeepsTemplate(t *testing.T) {
	got := MsgLocale(LocaleZhCN, "user.password.retry.limit.count")
	if got != "密码输入错误%v次" {
		t.Errorf("MsgLocale(无参) = %q, 期望保留原始模板", got)
	}
	if strings.Contains(got, "MISSING") {
		t.Errorf("无参调用不应产生 MISSING 标记: %q", got)
	}
}

// TestNamedPlaceholdersPassThrough 验证 {min}/{max} 原样输出。
func TestNamedPlaceholdersPassThrough(t *testing.T) {
	got := MsgLocale(LocaleZhCN, "length.not.valid")
	if got != "长度必须在{min}到{max}个字符之间" {
		t.Errorf("MsgLocale = %q, {min}/{max} 应原样保留", got)
	}
}

// TestLookupFallbackChain 验证查找回落链：未知地区 → 语言级 → 默认语言。
func TestLookupFallbackChain(t *testing.T) {
	cases := []struct {
		name string
		loc  Locale
		code string
		want string
	}{
		// en-gb 没有专属词条，退到 en 默认而非中文。
		{"未知英文地区退到英文", "en-gb", "user.logout.success", "Exit successful"},
		{"含脚本子标签退到中文", "zh-hans-cn", "user.logout.success", "退出成功"},
		{"未知语言退到默认", "fr-fr", "user.logout.success", "退出成功"},
		{"空 locale 退到默认", "", "user.logout.success", "退出成功"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MsgLocale(tc.loc, tc.code); got != tc.want {
				t.Errorf("MsgLocale(%q, %q) = %q, 期望 %q", tc.loc, tc.code, got, tc.want)
			}
		})
	}
}

// TestNoExportedMutationAPI 验证词条表条数固定。
func TestNoExportedMutationAPI(t *testing.T) {
	// 词条条数固定，用它间接确认没有别处往表里塞东西。
	if len(messagesZhCN) != len(messagesEnUS) {
		t.Fatalf("两份词条条数不等: zh=%d en=%d", len(messagesZhCN), len(messagesEnUS))
	}
	if len(catalogs) != 2 {
		t.Errorf("catalogs 应只含 2 种语言, 实际 %d —— 新增语言时请同步更新 langFallback",
			len(catalogs))
	}
}
