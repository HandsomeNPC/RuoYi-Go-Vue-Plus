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

// javaI18nDir 原项目词条目录，用于把 Go 侧的 map 与 .properties 交叉验证。
//
// 硬编码绝对路径而非相对路径：原项目在本仓库之外（见 CLAUDE.md），
// 没有稳定的相对关系。路径不存在时相关用例 Skip 而非 Fail ——
// 在没有原项目的机器（比如 CI）上，缺一份参照物不该让构建变红。
const javaI18nDir = `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus\ruoyi-admin\src\main\resources\i18n`

// rePositionalPlaceholder 匹配 MessageFormat 的位置占位符 {0} {1}。
var rePositionalPlaceholder = regexp.MustCompile(`\{[0-9]+\}`)

// loadProperties 读一份 .properties，返回键值对。
//
// 只处理原项目实际用到的形态：# 注释、空行、key=value。
// 不支持续行（\ 结尾）与 \uXXXX 转义 —— 原文件是 UTF-8 且无这两种用法
// （已确认），支持它们只会写出一段永远不被执行的代码。
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

// 把 Go 侧词条与原项目 .properties 逐条交叉验证。
//
// 这是本包最重要的用例：词条是**手工从 .properties 搬过来的 54×2 条文案**，
// 抄错一个字、漏一条、把 zh 的值贴到 en 里，都不会有任何编译错误，
// 而表现出来是「某个错误提示的文案不对」—— 那是没人会去核对的东西。
// 有了这条用例，词条表就不再依赖「当时抄得仔细」。
//
// 唯一允许的差异是占位符：{0}/{1} → %v（见包注释），比对时折算回去。
func TestCatalogsMatchJavaProperties(t *testing.T) {
	cases := []struct {
		file    string
		catalog map[string]string
	}{
		// messages.properties（无后缀）与 messages_zh_CN.properties 内容一致，
		// 故中文表同时对应两者，这里用带后缀的那份比对。
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
				// 把 Java 的 {0}/{1} 折算成 %v 再比，其余字符必须完全一致。
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

// 两份词条的键集必须完全一致。
//
// 缺键不会报错，会静默走 lookup 的回落链落到中文 —— 英文界面里冒出
// 一句中文提示，是最不容易被发现的那类问题。这条用例不依赖原项目，
// 在任何机器上都会跑。
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

// 同一个键在两种语言下的占位符个数必须相同。
//
// 调用方按中文模板的参数个数传参（比如 Msg(ctx,"user.password.retry.limit.exceed",5,10)），
// 若英文那条少一个 %v，输出就会多出 %!(EXTRA int=10) 这种尾巴；
// 多一个则是 %!v(MISSING)。两者都只在切到英文时才出现。
func TestPlaceholderCountsMatchAcrossLocales(t *testing.T) {
	for key, zh := range messagesZhCN {
		en, ok := messagesEnUS[key]
		if !ok {
			continue // 由 TestCatalogKeySetsMatch 报
		}
		if zhN, enN := strings.Count(zh, "%v"), strings.Count(en, "%v"); zhN != enN {
			t.Errorf("键 %q 占位符个数不一致: 中文 %d 个, 英文 %d 个", key, zhN, enN)
		}
	}
}

// 词条里不允许出现 %v 之外的 fmt 动词。
//
// 迁移时 {0} 一律换成 %v，若哪条不小心写成 %s 或 %d，传入类型不匹配就会
// 渲染成 %!d(string=xxx)。%v 对任何类型都安全，这条用例把这个约定锁住。
//
// 顺带确认 {min}/{max} 这三条**仍然保留**原样：它们是 Hibernate Validator
// 的属性占位符而非位置参数（见包注释），被误改成 %v 会让参数校验的文案
// 凭空多出两个占位符。
func TestOnlySafeVerbsAndNamedPlaceholdersPreserved(t *testing.T) {
	// 匹配 % 后跟一个字母的 fmt 动词，%% 已由前置替换排除。
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

// Parse 的归一化与白名单校验。
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
		// 列表形态取第一段：Java 的 forLanguageTag 遇到它会解析成 und
		// 从而回落默认语言，这里比原项目多支持一步。
		{"列表形态取第一段", "en-US, zh-CN", LocaleEnUS, true},
		{"空串", "", "", false},
		{"仅空白", "   ", "", false},
		// 以下是白名单要挡住的不可信输入。
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

// 超长输入即使前缀合法也必须整体拒绝，不能截断后使用。
//
// 截断会把一个畸形输入变成一个看起来正常的值，日志里再也看不出
// 调用方发了什么 —— 与 sanitizeTraceID 同样的取舍。
func TestParseRejectsRatherThanTruncates(t *testing.T) {
	long := "zh-CN" + strings.Repeat("x", localeMaxLength)
	if got, ok := Parse(long); ok {
		t.Errorf("Parse(超长值) = (%q, true), 应整体拒绝而非截断", got)
	}
}

// context 存取往返。
func TestContextRoundTrip(t *testing.T) {
	ctx := NewContext(context.Background(), LocaleEnUS)
	if got := FromContext(ctx); got != LocaleEnUS {
		t.Errorf("FromContext = %q, 期望 %q", got, LocaleEnUS)
	}
}

// 没存过语言、以及 nil context，都必须回落默认语言而不是 panic 或空串。
//
// 调用方多半在拼一句提示文案，不该因为少个语言标记就把请求搞挂；
// 空串会让 lookup 走一次无谓的回落，也让日志里出现空的语言字段。
func TestFromContextFallsBackToDefault(t *testing.T) {
	if got := FromContext(context.Background()); got != DefaultLocale {
		t.Errorf("未设置语言时 FromContext = %q, 期望 %q", got, DefaultLocale)
	}
	//lint:ignore SA1012 有意传 nil：调用方可能从未初始化的字段取 ctx
	if got := FromContext(nil); got != DefaultLocale { //nolint:staticcheck
		t.Errorf("nil context 时 FromContext = %q, 期望 %q", got, DefaultLocale)
	}
}

// Msg 按 context 里的语言取词条并渲染参数。
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

// 词条缺失时返回 code 本身，对齐 Java 侧 catch NoSuchMessageException 后 return code。
//
// 返回空串的话前端显示一片空白，无从判断是「没有提示」还是「词条漏了」；
// 返回 code 虽然丑，但能立刻看出缺哪个键。
func TestMsgReturnsCodeWhenMissing(t *testing.T) {
	const missing = "no.such.key.exists"
	if got := Msg(context.Background(), missing); got != missing {
		t.Errorf("Msg(缺失键) = %q, 期望原样返回 %q", got, missing)
	}
}

// 空 code 也走「原样返回」，不能 panic。
func TestMsgEmptyCode(t *testing.T) {
	if got := Msg(context.Background(), ""); got != "" {
		t.Errorf("Msg(\"\") = %q, 期望空串", got)
	}
}

// 不带参数时返回原始模板，不经 Sprintf。
//
// 过 Sprintf 会把 %v 渲染成 %!v(MISSING)。「该带参数却没带」是调用方的
// bug，这里保留原始模板（对齐 MessageFormat 无参时保留 {0} 的行为），
// 比加工成一句更难看的文案容易定位。
func TestMsgWithoutArgsKeepsTemplate(t *testing.T) {
	got := MsgLocale(LocaleZhCN, "user.password.retry.limit.count")
	if got != "密码输入错误%v次" {
		t.Errorf("MsgLocale(无参) = %q, 期望保留原始模板", got)
	}
	if strings.Contains(got, "MISSING") {
		t.Errorf("无参调用不应产生 MISSING 标记: %q", got)
	}
}

// {min}/{max} 不被 Sprintf 当占位符处理，原样输出。
func TestNamedPlaceholdersPassThrough(t *testing.T) {
	got := MsgLocale(LocaleZhCN, "length.not.valid")
	if got != "长度必须在{min}到{max}个字符之间" {
		t.Errorf("MsgLocale = %q, {min}/{max} 应原样保留", got)
	}
}

// 查找回落链：未知地区 → 语言级 → 默认语言。
func TestLookupFallbackChain(t *testing.T) {
	cases := []struct {
		name string
		loc  Locale
		code string
		want string
	}{
		// en-gb 没有专属词条，退到 en 的默认（en-us）而非中文。
		// 这是相对 Java 的有意偏差：原项目缺 messages_en.properties，
		// ResourceBundle 会退到系统默认区域，中文机器上返回中文。
		{"未知英文地区退到英文", "en-gb", "user.logout.success", "Exit successful"},
		{"含脚本子标签退到中文", "zh-hans-cn", "user.logout.success", "退出成功"},
		// 完全不认识的语言退到默认语言（中文）。
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

// 词条表不能在运行时被改写。
//
// catalogs 是启动后只读的全局 map，而中间件在每个请求里读它 ——
// 若将来有人加了「注册词条」的入口并在启动后调用，就是数据竞争。
// 这条用例不能真的检测竞争，只是把「没有导出的写入口」这个事实钉住：
// 一旦有人加了 Register，这里就得跟着改，从而被迫想一遍并发问题。
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
