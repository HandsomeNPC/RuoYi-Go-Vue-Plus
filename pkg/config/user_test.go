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

	if got, want := cfg.User, DefaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("缺 user 段时的配置与 DefaultUser() 不一致\ngot  = %+v\nwant = %+v", got, want)
	}
}

// TestRealYAMLUserMatchesDefaults 仓库 application.yaml 的 user 段应等于默认值。
func TestRealYAMLUserMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	if got, want := cfg.User, DefaultUser(); !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 user 段与 DefaultUser() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// TestUserDefaultsMatchJavaValues 默认值字面量断言。
func TestUserDefaultsMatchJavaValues(t *testing.T) {
	d := DefaultUser()
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
