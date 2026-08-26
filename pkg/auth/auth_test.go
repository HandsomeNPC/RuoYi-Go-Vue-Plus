package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key"

func TestClaimsSnowflakeIDSurvivesRoundTrip(t *testing.T) {
	const userID = int64(1761100000000000001)
	const deptID = int64(1761000000000000103)

	token, err := Sign(&Claims{UserID: userID, DeptID: deptID}, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	got, err := Verify(token, testSecret)
	if err != nil {
		t.Fatalf("Verify 失败: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("UserID 精度丢失: 期望 %d, 得到 %d", userID, got.UserID)
	}
	if got.DeptID != deptID {
		t.Errorf("DeptID 精度丢失: 期望 %d, 得到 %d", deptID, got.DeptID)
	}

	var loose jwt.MapClaims
	if _, err := jwt.ParseWithClaims(token, &loose,
		func(*jwt.Token) (any, error) { return []byte(testSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}),
	); err != nil {
		t.Fatalf("以 MapClaims 解析失败: %v", err)
	}
	if f, ok := loose["userId"].(float64); !ok {
		t.Fatalf("期望 MapClaims 把 userId 解成 float64, 得到 %T", loose["userId"])
	} else if int64(f) == userID {
		t.Errorf("前提已失效: MapClaims 竟未损坏雪花 id (%d)，"+
			"请重新评估 Claims 用具体结构体的必要性", int64(f))
	}
}

func TestVerifyRejectsAlgNone(t *testing.T) {
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{UserID: 1}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("构造 alg=none token 失败: %v", err)
	}

	if _, err := Verify(unsigned, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("alg=none 的 token 必须被拒: 得到 err=%v", err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	token, err := Sign(&Claims{UserID: 1, Username: "admin"}, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	forged, err := Sign(&Claims{UserID: 1, Username: "admin"}, "another-secret", time.Hour)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if _, err := Verify(forged, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("异密钥签发的 token 必须被拒: 得到 err=%v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token 格式异常: %q", token)
	}
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := Verify(tampered, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("载荷被篡改的 token 必须被拒: 得到 err=%v", err)
	}
}

func TestVerifyDistinguishesExpiredFromInvalid(t *testing.T) {
	expired, err := Sign(&Claims{UserID: 1}, testSecret, -time.Minute)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	if _, err := Verify(expired, testSecret); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("过期 token 应返回 ErrTokenExpired: 得到 %v", err)
	}
	if _, err := Verify("not-a-jwt", testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("畸形 token 应返回 ErrTokenInvalid: 得到 %v", err)
	}
	if _, err := Verify("", testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("空 token 应返回 ErrTokenInvalid: 得到 %v", err)
	}
}

func TestSignRejectsEmptySecret(t *testing.T) {
	if _, err := Sign(&Claims{UserID: 1}, "", time.Hour); err == nil {
		t.Error("空密钥必须报错")
	}
}

func TestTrimTokenPrefix(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		prefix string
		want   string
	}{
		{"标准形态", "Bearer abc.def.ghi", "Bearer", "abc.def.ghi"},
		{"大小写不敏感", "bearer abc", "Bearer", "abc"},
		{"全大写", "BEARER abc", "Bearer", "abc"},
		{"裸 token 原样返回", "abc.def.ghi", "Bearer", "abc.def.ghi"},
		{"前缀为空则原样返回", "Bearer abc", "", "Bearer abc"},
		{"空值", "", "Bearer", ""},
		{"前缀后无空格不剥离", "BearerXXX", "Bearer", "BearerXXX"},
		{"两侧空白", "  Bearer   abc  ", "Bearer", "abc"},
		{"仅有前缀", "Bearer", "Bearer", "Bearer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimTokenPrefix(tt.value, tt.prefix); got != tt.want {
				t.Errorf("TrimTokenPrefix(%q, %q) = %q, 期望 %q",
					tt.value, tt.prefix, got, tt.want)
			}
		})
	}
}
