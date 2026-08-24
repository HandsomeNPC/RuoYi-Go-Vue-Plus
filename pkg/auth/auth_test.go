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

// TestClaimsSnowflakeIDSurvivesRoundTrip 锁住「Claims 必须是具体结构体」这个决定。
//
// 这是本包最值得锁的一条。雪花 id 是 19 位十进制、超过 2^53，若 claims 走
// jwt.MapClaims（底层 map[string]any，数字解成 float64，仅 53 位有效位），
// 尾数会被**静默**抹掉 —— 拿改坏的 userId 查库，查不到算走运，查到别人是事故。
//
// 用例同时断言 MapClaims 确实会损坏该值：后半条是为了让前提失效得明显。
// 将来若有人把 Claims 改回 map 形态，这条会以「id 对不上」的形式立刻报错。
func TestClaimsSnowflakeIDSurvivesRoundTrip(t *testing.T) {
	// 取原项目种子数据里的超管 id，19 位，> 2^53(9007199254740992)。
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

	// 前提校验：确认 MapClaims 这条路真的会损坏该值。若某天这里不再失败，
	// 说明库的行为变了，上面那条断言的意义也需要重新评估。
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

// TestVerifyRejectsAlgNone 锁住 jwt.WithValidMethods。
//
// 不限定算法时，库信任 token 自己 header 里声明的 alg，攻击者改成 "none"
// 即可免签名 —— 这是 JWT 最经典的绕过方式。
func TestVerifyRejectsAlgNone(t *testing.T) {
	// jwt.UnsafeAllowNoneSignatureType 是库为构造此类 token 提供的哨兵值。
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{UserID: 1}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("构造 alg=none token 失败: %v", err)
	}

	if _, err := Verify(unsigned, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("alg=none 的 token 必须被拒: 得到 err=%v", err)
	}
}

// TestVerifyRejectsTamperedSignature 篡改载荷后签名对不上，必须拒绝。
func TestVerifyRejectsTamperedSignature(t *testing.T) {
	token, err := Sign(&Claims{UserID: 1, Username: "admin"}, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}

	// 换一个密钥签的 token 等价于伪造。
	forged, err := Sign(&Claims{UserID: 1, Username: "admin"}, "another-secret", time.Hour)
	if err != nil {
		t.Fatalf("Sign 失败: %v", err)
	}
	if _, err := Verify(forged, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("异密钥签发的 token 必须被拒: 得到 err=%v", err)
	}

	// 直接改动 payload 段。
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token 格式异常: %q", token)
	}
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	if _, err := Verify(tampered, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("载荷被篡改的 token 必须被拒: 得到 err=%v", err)
	}
}

// TestVerifyDistinguishesExpiredFromInvalid 过期与非法要能区分。
//
// 两者的提示文案不同（对齐 Java SaTokenExceptionHandler 的 switch），
// 混成一句会让「token 被篡改」与「正常用到过期」在日志里无法区分。
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

// TestSignRejectsEmptySecret 空密钥签出的 token 谁都能伪造，且看起来与正常的无异。
func TestSignRejectsEmptySecret(t *testing.T) {
	if _, err := Sign(&Claims{UserID: 1}, "", time.Hour); err == nil {
		t.Error("空密钥必须报错")
	}
}

// TestPasswordVerifiesJavaBCryptHash 跨语言验证：直接拿原项目种子数据里的
// 哈希校验已知明文。
//
// 这是唯一能真正证明「Go 的 bcrypt 与 hutool 的 BCrypt 兼容」的用例 ——
// 往返测试（自己哈希自己校验）做不到这件事：变体选错（$2a$/$2y$）、
// cost 不同都能自洽地往返成功，却与原项目的存量密码完全对不上。
// 与 pkg/encrypt 拿 FIPS-197 向量比对是同一个思路。
//
// 哈希取自 script/sql/ry_vue.sql 的 sys_user 种子数据。
func TestPasswordVerifiesJavaBCryptHash(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hash     string
	}{
		{
			// admin 用户，原项目文档给出的密码是 admin123。
			name:     "admin",
			password: "admin123",
			hash:     "$2a$10$7JB720yubVSZvUI0rEqK/.VqGOZTH.ulu33dHOiBE8ByOhJIrdAu2",
		},
		{
			// test / test1 用户，nick_name 列里直接写明「密码666666」。
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

// TestHashPasswordRoundTrip 自产哈希可自校验，且格式与原项目一致（$2a$10$）。
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

// TestVerifyPasswordEmptyHash sys_user.password 列默认空串，
// 没设过密码的账号不该因为传空密码就登录成功。
func TestVerifyPasswordEmptyHash(t *testing.T) {
	if err := VerifyPassword("", ""); !errors.Is(err, ErrPasswordMismatch) {
		t.Errorf("空哈希应判为不匹配: 得到 %v", err)
	}
}

// TestVerifyPasswordMalformedHashIsNotMismatch 哈希串格式非法是**数据问题**，
// 不是密码错误 —— 调用方据此决定不计入密码错误次数，否则会掩盖真正的原因。
func TestVerifyPasswordMalformedHashIsNotMismatch(t *testing.T) {
	// 迁移时漏加密的明文密码就是这个形态。
	err := VerifyPassword("admin123", "admin123")
	if err == nil {
		t.Fatal("非法哈希串不该校验通过")
	}
	if errors.Is(err, ErrPasswordMismatch) {
		t.Error("非法哈希串应返回格式错误而非 ErrPasswordMismatch")
	}
}

// TestHashPasswordRejectsOverlongInput bcrypt 只取前 72 字节，
// 静默截断会让两个不同的长密码等价而调用方毫无察觉。
func TestHashPasswordRejectsOverlongInput(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); err == nil {
		t.Error("超过 72 字节的密码必须报错")
	}
}

// TestLoginIDRoundTrip LoginID 与 ParseLoginID 互逆。
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

// TestLoginIDRejectsIncomplete 缺 userType 或 userID 时返回 ok=false 而非 panic。
//
// Java 侧这两种情况抛 IllegalArgumentException；Go 侧交调用方处置，
// 不让一个畸形的登录请求打挂进程。
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

// TestParseLoginIDRejectsMalformed 畸形输入一律 ok=false。
func TestParseLoginIDRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "sys_user", "sys_user:", ":1", "sys_user:abc", "sys_user:0"} {
		if _, _, ok := ParseLoginID(in); ok {
			t.Errorf("ParseLoginID(%q) 应失败", in)
		}
	}
}

// TestParseLoginIDSplitsOnFirstColon 按第一个冒号切分，不是最后一个。
//
// 用户类型里不含冒号，而将来 ID 段若拼上设备标识，从后切会把类型段割坏。
func TestParseLoginIDSplitsOnFirstColon(t *testing.T) {
	userType, _, ok := ParseLoginID("sys_user:123:extra")
	if ok {
		t.Errorf("ID 段非纯数字应失败, 却解析出 userType=%q", userType)
	}
}

// TestTrimTokenPrefix 前缀剥离的边界。
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
		// 关键边界：不连带空格一起比的话，BearerXXX 会被切成 XXX。
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

// TestSuperAdminIDMatchesConstant 本包为保持无内部依赖重新声明了超管 id，
// 与 pkg/constant 的定义必须一致 —— 两处分叉会让「谁是超管」有两个答案。
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
