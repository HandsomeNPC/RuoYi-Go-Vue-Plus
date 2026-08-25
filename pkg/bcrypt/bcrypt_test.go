package bcrypt

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifyAcceptsJavaBCryptHash(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hash     string
	}{
		{
			name:     "admin",
			password: "admin123",
			hash:     "$2a$10$7JB720yubVSZvUI0rEqK/.VqGOZTH.ulu33dHOiBE8ByOhJIrdAu2",
		},
		{
			name:     "test",
			password: "666666",
			hash:     "$2a$10$b8yUzN0C71sbz.PhNOCgJe.Tu1yWC3RNrTyjSQ8p1W0.aaUXUJ.Ne",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Verify(tt.password, tt.hash); err != nil {
				t.Errorf("Java 侧 BCrypt 哈希校验失败(跨语言不兼容): %v", err)
			}
			if err := Verify("wrong-password", tt.hash); !errors.Is(err, ErrPasswordMismatch) {
				t.Errorf("错误密码应返回 ErrPasswordMismatch: 得到 %v", err)
			}
			if !Checkpw(tt.password, tt.hash) {
				t.Error("Checkpw 对正确密码应返回 true")
			}
			if Checkpw("wrong-password", tt.hash) {
				t.Error("Checkpw 对错误密码应返回 false")
			}
		})
	}
}

func TestHashpwRoundTrip(t *testing.T) {
	const pw = "s3cret-pass"

	hashed, err := Hashpw(pw)
	if err != nil {
		t.Fatalf("Hashpw 失败: %v", err)
	}
	if !strings.HasPrefix(hashed, "$2a$10$") {
		t.Errorf("哈希前缀应与原项目一致($2a$10$): 得到 %q", hashed)
	}
	if len(hashed) != 60 {
		t.Errorf("bcrypt 哈希应为 60 字符: 得到 %d", len(hashed))
	}
	if err := Verify(pw, hashed); err != nil {
		t.Errorf("自产哈希校验失败: %v", err)
	}
}

func TestVerifyEmptyHash(t *testing.T) {
	if err := Verify("", ""); !errors.Is(err, ErrPasswordMismatch) {
		t.Errorf("空哈希应判为不匹配: 得到 %v", err)
	}
}

func TestVerifyMalformedHashIsNotMismatch(t *testing.T) {
	err := Verify("admin123", "admin123")
	if err == nil {
		t.Fatal("非法哈希串不该校验通过")
	}
	if errors.Is(err, ErrPasswordMismatch) {
		t.Error("非法哈希串应返回格式错误而非 ErrPasswordMismatch")
	}
}

func TestHashpwRejectsOverlongInput(t *testing.T) {
	if _, err := Hashpw(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("超过 72 字节的密码应返回 ErrPasswordTooLong: 得到 %v", err)
	}
}

func TestHashpwWithCostProducesMatchingHash(t *testing.T) {
	const pw = "cost-check"

	hashed, err := HashpwWithCost(pw, 4)
	if err != nil {
		t.Fatalf("HashpwWithCost 失败: %v", err)
	}
	if !strings.HasPrefix(hashed, "$2a$04$") {
		t.Errorf("哈希前缀应反映指定 cost: 得到 %q", hashed)
	}
	if !Checkpw(pw, hashed) {
		t.Error("指定 cost 的自产哈希应可校验通过")
	}
}
