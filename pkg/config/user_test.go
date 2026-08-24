package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestUserDefaultsWhenAbsent 缺 user 段时应回落到默认值。
func TestUserDefaultsWhenAbsent(t *testing.T) {
	path := writeYAML(t, fullYAML)
	cfg := mustLoad(t, path)

	if got, want := cfg.User, defaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("缺 user 段时的配置与 defaultUser() 不一致\ngot  = %+v\nwant = %+v", got, want)
	}
}

// TestRealYAMLUserMatchesDefaults 仓库 application.yaml 的 user 段应等于默认值。
func TestRealYAMLUserMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	if got, want := cfg.User, defaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 user 段与 defaultUser() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// TestUserDefaultsMatchJavaValues 默认值字面量断言。
func TestUserDefaultsMatchJavaValues(t *testing.T) {
	d := defaultUser()
	if got, want := d.Password.MaxRetryCount, 5; got != want {
		t.Errorf("maxRetryCount = %d, want %d", got, want)
	}
	if got, want := d.Password.LockTime, 10; got != want {
		t.Errorf("lockTime = %d, want %d", got, want)
	}
	if got, want := d.Password.Lock(), 10*time.Minute; got != want {
		t.Errorf("Lock() = %v, want %v", got, want)
	}
}

// TestUserValidateRejectsNonPositive 两项都必须为正数。
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

// TestAuthExcludesCoverLoginEndpoints 鉴权免登名单必须包含 /auth/**。
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

// TestAuthExcludesMatchJavaSecurityExcludes 免登名单集合断言。
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
			t.Errorf("缺少 security.excludes 里的 %q", want)
		}
	}
	if len(defaultAuthExcludes) != len(javaExcludes)+1 {
		t.Errorf("defaultAuthExcludes 有 %d 条，期望 %d 条: %v",
			len(defaultAuthExcludes), len(javaExcludes)+1, defaultAuthExcludes)
	}
}

// TestAuthValidateRejectsEmptyExclude 免登名单里不能有空串。
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

// TestAuthHeaderConstantsMatchJava 头名常量断言。
func TestAuthHeaderConstantsMatchJava(t *testing.T) {
	if got, want := TokenHeader, "Authorization"; got != want {
		t.Errorf("TokenHeader = %q, want %q", got, want)
	}
	if got, want := TokenPrefix, "Bearer"; got != want {
		t.Errorf("TokenPrefix = %q, want %q", got, want)
	}
	if got, want := ClientIDHeader, "clientid"; got != want {
		t.Errorf("ClientIDHeader = %q, want %q", got, want)
	}
}
