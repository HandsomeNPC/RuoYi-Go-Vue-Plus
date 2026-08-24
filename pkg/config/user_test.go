package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// 缺 user 段时应回落到默认值。
//
// 与 middleware 段同一个理由：viper 对缺失的键给零值，而 MaxRetryCount 的
// 零值不是「不限制」而是**恒定锁死**（「已错次数 >= 0」在第一次尝试就成立），
// yaml 里漏写这段会让谁都登不进来。铺默认值让「没写」与「写了默认值」走同一条路。
func TestUserDefaultsWhenAbsent(t *testing.T) {
	// fullYAML 只有 server/datasource/redis/jwt，没有 user 段。
	path := writeYAML(t, fullYAML)
	cfg := mustLoad(t, path)

	if got, want := cfg.User, defaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("缺 user 段时的配置与 defaultUser() 不一致\ngot  = %+v\nwant = %+v", got, want)
	}
}

// 仓库真实 application.yaml 的 user 段写的是默认值的显式重述。
//
// 与 TestRealYAMLMatchesDefaults 同一个用意：yaml 是安全的文档 ——
// 照着改能预期行为，删掉整段也不会改行为。
func TestRealYAMLUserMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	if got, want := cfg.User, defaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 user 段与 defaultUser() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// 默认值必须对齐原项目 application.yml:38-43（maxRetryCount: 5 / lockTime: 10）。
//
// 单独断言字面量而非只比结构体：上面两条只保证「三处一致」，
// 若有人把默认值和 yaml 一起改错，那两条依然会通过。
func TestUserDefaultsMatchJavaValues(t *testing.T) {
	d := defaultUser()
	if got, want := d.Password.MaxRetryCount, 5; got != want {
		t.Errorf("maxRetryCount = %d, 原项目为 %d", got, want)
	}
	if got, want := d.Password.LockTime, 10; got != want {
		t.Errorf("lockTime = %d, 原项目为 %d", got, want)
	}
	if got, want := d.Password.Lock(), 10*time.Minute; got != want {
		t.Errorf("Lock() = %v, want %v", got, want)
	}
}

// 两项都必须为正数。
//
// 零或负数在这里不是「不限制」而是**恒定锁死**：MaxRetryCount<=0 时
// 「已错次数 >= 上限」在第一次尝试就成立，谁都登不进去。
// 想关掉重试限制不该靠填 0 —— 那需要一个显式的开关。
func TestUserValidateRejectsNonPositive(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		key  string
	}{
		{"maxRetryCount 为 0", "\nuser:\n  password:\n    maxRetryCount: 0\n", "maxRetryCount"},
		{"maxRetryCount 为负", "\nuser:\n  password:\n    maxRetryCount: -1\n", "maxRetryCount"},
		{"lockTime 为 0", "\nuser:\n  password:\n    lockTime: 0\n", "lockTime"},
		{"lockTime 为负", "\nuser:\n  password:\n    lockTime: -5\n", "lockTime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, fullYAML+tt.yaml)
			err := loadErr(t, path)
			if err == nil {
				t.Fatal("应校验失败")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("错误信息应点明是哪一项(%s): %v", tt.key, err)
			}
		})
	}
}

// 鉴权免登名单必须包含 /auth/**。
//
// 这是**相对原项目的必要新增**：Java 侧 AuthController 整个类挂 @SaIgnore
// 免鉴权，Go 没有注解机制只能进配置名单。漏了它，登录接口自己就需要 token ——
// 谁也登不进来，且症状（登录返回 401）会让人去查密码而不是查这份名单。
func TestAuthExcludesCoverLoginEndpoints(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	var found bool
	for _, p := range cfg.Middleware.Auth.Excludes {
		if p == "/auth/**" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("middleware.auth.excludes 必须含 /auth/**，否则登录接口自己也要 token: %v",
			cfg.Middleware.Auth.Excludes)
	}
}

// 免登名单前 11 条必须逐字对齐原项目 security.excludes（application.yml:100-113）。
//
// 这是安全配置，少一条会让本该公开的静态资源变成 401，多一条则是敞开的口子。
// 逐字列出而非只数个数：顺序无关紧要，但**集合**必须一致。
func TestAuthExcludesMatchJavaSecurityExcludes(t *testing.T) {
	javaExcludes := []string{
		"/*.html",
		"/**/*.html",
		"/**/*.css",
		"/**/*.js",
		"/favicon.ico",
		"/error",
		"/*/api-docs",
		"/*/api-docs/**",
		"/warm-flow-ui/config",
		"/snail-chat/**",
		"/api/snail/chat/**",
	}

	got := make(map[string]bool, len(defaultAuthExcludes))
	for _, p := range defaultAuthExcludes {
		got[p] = true
	}
	for _, want := range javaExcludes {
		if !got[want] {
			t.Errorf("缺少原项目 security.excludes 里的 %q", want)
		}
	}
	// Go 侧只应比 Java 多 /auth/**（@SaIgnore 的替代物）。
	if len(defaultAuthExcludes) != len(javaExcludes)+1 {
		t.Errorf("defaultAuthExcludes 有 %d 条，期望 %d 条(原项目 %d 条 + /auth/**)。"+
			"新增免鉴权路径是放宽安全边界，请连同本用例一起更新: %v",
			len(defaultAuthExcludes), len(javaExcludes)+1, len(javaExcludes), defaultAuthExcludes)
	}
}

// 免登名单里不能有空串。
//
// MatchAnyPath 对空 pattern 只匹配空路径，看似无害，但它在配置文件里的形态
// 是一条 `- ""` 或笔误留下的空行 —— 写的人多半以为自己排除了什么。
func TestAuthValidateRejectsEmptyExclude(t *testing.T) {
	path := writeYAML(t, fullYAML+"\nmiddleware:\n  auth:\n    excludes:\n      - \"\"\n")
	err := loadErr(t, path)
	if err == nil {
		t.Fatal("含空路径的 excludes 应校验失败")
	}
	if !strings.Contains(err.Error(), "excludes") {
		t.Errorf("错误信息应点明是 excludes: %v", err)
	}
}

// 头名常量必须对齐原项目。
//
// 这三个是与前端的协议，改一个字就是接口不通，而症状（token 传了却说没登录）
// 与真正的原因隔得很远。
func TestAuthHeaderConstantsMatchJava(t *testing.T) {
	// sa-token 的 token-name（application.yml:91）。
	if got, want := TokenHeader, "Authorization"; got != want {
		t.Errorf("TokenHeader = %q, 原项目为 %q", got, want)
	}
	// sa-token 的 token-prefix（common-satoken.yml）。
	if got, want := TokenPrefix, "Bearer"; got != want {
		t.Errorf("TokenPrefix = %q, 原项目为 %q", got, want)
	}
	// LoginHelper.CLIENT_KEY（LoginHelper.java:43）。
	if got, want := ClientIDHeader, "clientid"; got != want {
		t.Errorf("ClientIDHeader = %q, 原项目为 %q", got, want)
	}
}
