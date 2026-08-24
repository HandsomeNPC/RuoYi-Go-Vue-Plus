package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ruoyi-go-vue-plus/pkg/constant"
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

func TestPasswordVerifiesJavaBCryptHash(t *testing.T) {
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
			if err := VerifyPassword(tt.password, tt.hash); err != nil {
				t.Errorf("Java 侧 BCrypt 哈希校验失败(跨语言不兼容): %v", err)
			}
			if err := VerifyPassword("wrong-password", tt.hash); !errors.Is(err, ErrPasswordMismatch) {
				t.Errorf("错误密码应返回 ErrPasswordMismatch: 得到 %v", err)
			}
		})
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	const pw = "s3cret-pass"

	hashed, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if !strings.HasPrefix(hashed, "$2a$10$") {
		t.Errorf("哈希前缀应与原项目一致($2a$10$): 得到 %q", hashed)
	}
	if len(hashed) != 60 {
		t.Errorf("bcrypt 哈希应为 60 字符: 得到 %d", len(hashed))
	}
	if err := VerifyPassword(pw, hashed); err != nil {
		t.Errorf("自产哈希校验失败: %v", err)
	}
}

func TestVerifyPasswordEmptyHash(t *testing.T) {
	if err := VerifyPassword("", ""); !errors.Is(err, ErrPasswordMismatch) {
		t.Errorf("空哈希应判为不匹配: 得到 %v", err)
	}
}

func TestVerifyPasswordMalformedHashIsNotMismatch(t *testing.T) {
	err := VerifyPassword("admin123", "admin123")
	if err == nil {
		t.Fatal("非法哈希串不该校验通过")
	}
	if errors.Is(err, ErrPasswordMismatch) {
		t.Error("非法哈希串应返回格式错误而非 ErrPasswordMismatch")
	}
}

func TestHashPasswordRejectsOverlongInput(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); err == nil {
		t.Error("超过 72 字节的密码必须报错")
	}
}

func TestLoginIDRoundTrip(t *testing.T) {
	u := &LoginUser{UserID: 1761100000000000001, UserType: UserTypeSys}

	id, ok := u.LoginID()
	if !ok {
		t.Fatal("LoginID 应成功")
	}
	if want := "sys_user:1761100000000000001"; id != want {
		t.Errorf("LoginID = %q, 期望 %q", id, want)
	}

	userType, userID, ok := ParseLoginID(id)
	if !ok {
		t.Fatal("ParseLoginID 应成功")
	}
	if userType != UserTypeSys || userID != u.UserID {
		t.Errorf("ParseLoginID = (%q, %d), 期望 (%q, %d)",
			userType, userID, UserTypeSys, u.UserID)
	}
}

func TestLoginIDRejectsIncomplete(t *testing.T) {
	cases := []struct {
		name string
		user *LoginUser
	}{
		{"nil", nil},
		{"缺 userType", &LoginUser{UserID: 1}},
		{"缺 userID", &LoginUser{UserType: UserTypeSys}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.user.LoginID(); ok {
				t.Error("应返回 ok=false")
			}
		})
	}
}

func TestParseLoginIDRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "sys_user", "sys_user:", ":1", "sys_user:abc", "sys_user:0"} {
		if _, _, ok := ParseLoginID(in); ok {
			t.Errorf("ParseLoginID(%q) 应失败", in)
		}
	}
}

func TestParseLoginIDSplitsOnFirstColon(t *testing.T) {
	userType, _, ok := ParseLoginID("sys_user:123:extra")
	if ok {
		t.Errorf("ID 段非纯数字应失败, 却解析出 userType=%q", userType)
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

func TestSuperAdminIDMatchesConstant(t *testing.T) {
	if superAdminUserID != constant.SuperAdminUserID {
		t.Errorf("auth.superAdminUserID(%d) 与 constant.SuperAdminUserID(%d) 不一致",
			superAdminUserID, constant.SuperAdminUserID)
	}

	u := &LoginUser{UserID: constant.SuperAdminUserID}
	if !u.IsSuperAdmin() {
		t.Error("超管 id 应判定为超级管理员")
	}
	if (&LoginUser{UserID: 2}).IsSuperAdmin() {
		t.Error("非超管 id 不应判定为超级管理员")
	}
}
