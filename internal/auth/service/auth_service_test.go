package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
)

// 原项目种子数据（script/sql/ry_vue.sql），照抄以便跨语言对照。
const (
	seedClientID = "e5cd7e4891bf95d1d19206ce24a7b32e" // md5("pc"+"pc123")
	seedUserID   = int64(1761100000000000001)
	// admin 的密码哈希，明文 admin123。由 hutool BCrypt 产出，
	// 本用例同时充当「Go 的 bcrypt 能读 Java 的哈希」的端到端验证。
	seedAdminHash = "$2a$10$7JB720yubVSZvUI0rEqK/.VqGOZTH.ulu33dHOiBE8ByOhJIrdAu2"
	seedPassword  = "admin123"
)

// fixture 一套自洽的登录测试环境。
//
// 用内存 SQLite + 内存 Redis 而非 mock：登录流程的价值恰恰在于
// repository 的 SQL、service 的顺序、Redis 的计数三者串起来的行为 ——
// 把 repository 换成 mock 就测不到 del_flag 过滤这类真正会出错的地方。
// SQLite 用的是纯 Go 驱动（glebarez/sqlite），不需要 CGO。
type fixture struct {
	svc *AuthService
	db  *gorm.DB
	mr  *miniredis.Miniredis
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Discard,
		// 与 pkg/database 保持一致：实体 SysUser -> 表 sys_user。
		NamingStrategy:         schema.NamingStrategy{SingularTable: true},
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&systemmodel.SysUser{}, &systemmodel.SysClient{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &config.Config{
		JWT:  config.JWT{Secret: "test-secret", ExpireMinutes: 720},
		User: config.User{Password: config.UserPassword{MaxRetryCount: 5, LockTime: 10}},
	}

	f := &fixture{svc: NewAuthService(db, rdb, cfg), db: db, mr: mr}
	f.seed(t)
	return f
}

// seed 灌入与原项目种子数据一致的一个客户端与一个用户。
func (f *fixture) seed(t *testing.T) {
	t.Helper()

	client := &systemmodel.SysClient{
		ID:            1762000000000000001,
		ClientID:      seedClientID,
		ClientKey:     "pc",
		ClientSecret:  "pc123",
		GrantType:     "password,social",
		DeviceType:    "pc",
		ActiveTimeout: 1800,
		Timeout:       604800,
		Status:        constant.StatusNormal,
		DelFlag:       constant.StatusNormal,
	}
	if err := f.db.Create(client).Error; err != nil {
		t.Fatalf("写入客户端失败: %v", err)
	}

	user := &systemmodel.SysUser{
		UserID:   seedUserID,
		DeptID:   1761000000000000103,
		UserName: "admin",
		NickName: "疯狂的狮子Li",
		UserType: auth.UserTypeSys,
		Password: seedAdminHash,
		Status:   constant.StatusNormal,
		DelFlag:  constant.StatusNormal,
	}
	if err := f.db.Create(user).Error; err != nil {
		t.Fatalf("写入用户失败: %v", err)
	}
}

// login 发起一次密码登录。
func (f *fixture) login(username, password string) (*authmodel.LoginVo, error) {
	return f.svc.Login(context.Background(), &authmodel.LoginBody{
		ClientID:  seedClientID,
		GrantType: "password",
		Username:  username,
		Password:  password,
	}, "127.0.0.1")
}

// errCount 读密码错误计数，键不存在时返回 -1。
func (f *fixture) errCount(t *testing.T, username string) int {
	t.Helper()
	v, err := f.mr.Get(constant.PwdErrCntKeyPrefix + username)
	if err != nil {
		return -1
	}
	n := 0
	for _, c := range v {
		n = n*10 + int(c-'0')
	}
	return n
}

// TestLoginSuccess 正常登录：返回 token 与客户端信息，会话已落 Redis。
//
// 密码用的是**原项目种子数据的 BCrypt 哈希**，所以这条也顺带端到端地验证了
// 「Go 的 bcrypt 能读 hutool 产出的哈希」——
// 跨语言不兼容会让全站存量用户都登不进来，值得在这一层再锁一次。
func TestLoginSuccess(t *testing.T) {
	f := newFixture(t)

	vo, err := f.login("admin", seedPassword)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	if vo.AccessToken == "" {
		t.Error("access_token 不该为空")
	}
	if vo.ClientID != seedClientID {
		t.Errorf("client_id = %q, 期望 %q", vo.ClientID, seedClientID)
	}
	// expire_in 取 sys_client.timeout（绝对超时），不是 activeTimeout。
	if vo.ExpireIn != 604800 {
		t.Errorf("expire_in = %d, 期望 604800(client.timeout)", vo.ExpireIn)
	}

	// token 必须能验签，且带上客户端绑定信息。
	claims, err := auth.Verify(vo.AccessToken, "test-secret")
	if err != nil {
		t.Fatalf("签发的 token 应能验签: %v", err)
	}
	if claims.UserID != seedUserID {
		t.Errorf("claims.UserID = %d, 期望 %d(雪花 id 不能丢精度)", claims.UserID, seedUserID)
	}
	if claims.ClientID != seedClientID {
		t.Errorf("claims.ClientID = %q, 期望 %q", claims.ClientID, seedClientID)
	}
	if want := "sys_user:1761100000000000001"; claims.Subject != want {
		t.Errorf("claims.Subject = %q, 期望 %q", claims.Subject, want)
	}

	// 会话与在线记录都要落 Redis。
	if _, err := f.mr.Get(auth.TokenKeyPrefix + vo.AccessToken); err != nil {
		t.Errorf("会话未写入 Redis: %v", err)
	}
	if _, err := f.mr.Get(constant.OnlineTokenKeyPrefix + vo.AccessToken); err != nil {
		t.Errorf("在线用户记录未写入 Redis: %v", err)
	}

	// 会话 TTL 取 activeTimeout（滑动空闲超时）。
	if got, want := f.mr.TTL(auth.TokenKeyPrefix+vo.AccessToken), 1800*time.Second; got != want {
		t.Errorf("会话 TTL = %v, 期望 %v(client.activeTimeout)", got, want)
	}
}

// TestLoginUpdatesLastLoginInfo 登录成功后写回 login_ip / login_date。
func TestLoginUpdatesLastLoginInfo(t *testing.T) {
	f := newFixture(t)

	if _, err := f.login("admin", seedPassword); err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	var user systemmodel.SysUser
	if err := f.db.Where("user_id = ?", seedUserID).First(&user).Error; err != nil {
		t.Fatalf("查用户失败: %v", err)
	}
	if user.LoginIP != "127.0.0.1" {
		t.Errorf("login_ip = %q, 期望 127.0.0.1", user.LoginIP)
	}
	if user.LoginDate == nil {
		t.Error("login_date 应被写入")
	}
	// 关键：更新登录信息不能顺手把密码清空。
	// Java 侧靠 @TableField(updateStrategy=NOT_EMPTY) 保证，
	// Go 侧靠 repository 只 Updates 指定列（用 Save 就会踩这个坑）。
	if user.Password != seedAdminHash {
		t.Errorf("更新登录信息不该动密码字段: password = %q", user.Password)
	}
}

// TestLoginNonexistentUserDoesNotIncrementRetryCount 锁住第 1 步与第 2 步的顺序。
//
// 这是本文件最重要的一条。「查用户」必须早于「密码错误计数」——
// 否则攻击者能用任意**不存在**的用户名把某个真实账号刷到锁定
// （计数键是按 username 存的，而「这个用户名存不存在」在锁定前是未知的）。
//
// 对齐 Java 的 loadUserByUsername 先行（PasswordAuthStrategy.java:69-70）。
func TestLoginNonexistentUserDoesNotIncrementRetryCount(t *testing.T) {
	f := newFixture(t)

	for i := 0; i < 10; i++ {
		if _, err := f.login("no-such-user", "whatever"); err == nil {
			t.Fatal("不存在的用户应登录失败")
		}
	}

	if got := f.errCount(t, "no-such-user"); got != -1 {
		t.Errorf("不存在的用户不该产生错误计数键, 得到计数 %d", got)
	}
	// 真实账号的计数也必须没被碰过。
	if got := f.errCount(t, "admin"); got != -1 {
		t.Errorf("admin 的计数不该被无关的用户名影响, 得到 %d", got)
	}
	// 真实账号仍能正常登录。
	if _, err := f.login("admin", seedPassword); err != nil {
		t.Errorf("admin 应仍能登录: %v", err)
	}
}

// TestLoginWrongPasswordIncrementsAndLocks 密码错误计数与锁定。
//
// 对应 Java SysLoginService.checkLogin：未达上限提示已错次数，
// 达到上限提示锁定分钟数，且此后**即使密码正确也拒绝**。
func TestLoginWrongPasswordIncrementsAndLocks(t *testing.T) {
	f := newFixture(t)

	// 前 4 次：提示已错次数。
	for i := 1; i <= 4; i++ {
		_, err := f.login("admin", "wrong-password")
		if err == nil {
			t.Fatalf("第 %d 次错误密码应失败", i)
		}
		if !strings.Contains(err.Error(), "密码输入错误") {
			t.Errorf("第 %d 次的提示应是「密码输入错误N次」: %v", i, err)
		}
		if got := f.errCount(t, "admin"); got != i {
			t.Errorf("第 %d 次后计数 = %d, 期望 %d", i, got, i)
		}
	}

	// 第 5 次：达到上限，提示锁定。
	_, err := f.login("admin", "wrong-password")
	if err == nil {
		t.Fatal("第 5 次应失败")
	}
	if !strings.Contains(err.Error(), "锁定") {
		t.Errorf("第 5 次的提示应含「锁定」: %v", err)
	}

	// 锁定后即使密码正确也拒绝 —— 这才是「锁定」的意义。
	if _, err := f.login("admin", seedPassword); err == nil {
		t.Error("锁定后即使密码正确也应拒绝")
	} else if !strings.Contains(err.Error(), "锁定") {
		t.Errorf("锁定期内的提示应含「锁定」: %v", err)
	}
}

// TestLoginRetryTTLSlides 锁住「滑动窗口」而非「固定窗口」。
//
// 原项目每次失败都重设 TTL（RedisUtils.setCacheObject(key, n, Duration.ofMinutes(lockTime))）——
// 即每 9 分钟错一次、错满 5 次跨越 40 分钟，依然会锁。
// 若做成固定窗口，那种慢速爆破就永远不会触发锁定。
func TestLoginRetryTTLSlides(t *testing.T) {
	f := newFixture(t)
	key := constant.PwdErrCntKeyPrefix + "admin"

	if _, err := f.login("admin", "wrong"); err == nil {
		t.Fatal("应失败")
	}
	if got, want := f.mr.TTL(key), 10*time.Minute; got != want {
		t.Fatalf("首次失败后 TTL = %v, 期望 %v", got, want)
	}

	// 消耗掉大半个窗口后再错一次，TTL 应被重设回完整的 10 分钟。
	f.mr.FastForward(9 * time.Minute)
	if _, err := f.login("admin", "wrong"); err == nil {
		t.Fatal("应失败")
	}
	if got, want := f.mr.TTL(key), 10*time.Minute; got != want {
		t.Errorf("再次失败后 TTL = %v, 期望重设为 %v(滑动窗口)", got, want)
	}
	if got := f.errCount(t, "admin"); got != 2 {
		t.Errorf("计数 = %d, 期望 2(未过期则累加)", got)
	}
}

// TestLoginSuccessClearsRetryCount 登录成功清零错误计数。
func TestLoginSuccessClearsRetryCount(t *testing.T) {
	f := newFixture(t)

	for i := 0; i < 3; i++ {
		if _, err := f.login("admin", "wrong"); err == nil {
			t.Fatal("应失败")
		}
	}
	if got := f.errCount(t, "admin"); got != 3 {
		t.Fatalf("前提不成立: 计数 = %d, 期望 3", got)
	}

	if _, err := f.login("admin", seedPassword); err != nil {
		t.Fatalf("登录应成功: %v", err)
	}
	if got := f.errCount(t, "admin"); got != -1 {
		t.Errorf("登录成功应清零计数, 得到 %d", got)
	}
}

// TestLoginDisabledUser 停用的账号不能登录，对应 user.blocked 词条。
func TestLoginDisabledUser(t *testing.T) {
	f := newFixture(t)

	if err := f.db.Model(&systemmodel.SysUser{}).
		Where("user_id = ?", seedUserID).
		Update("status", constant.StatusDisable).Error; err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}

	_, err := f.login("admin", seedPassword)
	if err == nil {
		t.Fatal("停用账号应登录失败")
	}
	if !strings.Contains(err.Error(), "禁用") {
		t.Errorf("提示应指明账号已禁用: %v", err)
	}
	// 停用与密码无关，不该计入错误次数。
	if got := f.errCount(t, "admin"); got != -1 {
		t.Errorf("停用账号不该产生密码错误计数, 得到 %d", got)
	}
}

// TestLoginDeletedUserNotFound 逻辑删除的用户查不到。
//
// Java 侧靠 @TableLogic 自动加 del_flag='0'，Go 侧靠 repository 的
// NotDeleted() scope —— **漏了它被删的用户还能登录**，而那不会报错。
func TestLoginDeletedUserNotFound(t *testing.T) {
	f := newFixture(t)

	if err := f.db.Model(&systemmodel.SysUser{}).
		Where("user_id = ?", seedUserID).
		Update("del_flag", "1").Error; err != nil {
		t.Fatalf("更新删除标志失败: %v", err)
	}

	if _, err := f.login("admin", seedPassword); err == nil {
		t.Fatal("已逻辑删除的用户不该能登录")
	}
}

// TestLoginGrantTypeExactMatchNotSubstring 锁住授权类型的精确比对。
//
// Java 侧用 StringUtils.contains 做**子串**匹配（AuthController.java:86），
// 于是 grantType="pass" 会命中 "password,social"。那是 bug ——
// 一个拼错的 grantType 通过校验后会在策略分派处才失败。
func TestLoginGrantTypeExactMatchNotSubstring(t *testing.T) {
	f := newFixture(t)

	for _, gt := range []string{"pass", "word", "sword", "password,social"} {
		_, err := f.svc.Login(context.Background(), &authmodel.LoginBody{
			ClientID:  seedClientID,
			GrantType: gt,
			Username:  "admin",
			Password:  seedPassword,
		}, "127.0.0.1")
		if err == nil {
			t.Errorf("grantType=%q 不是完整取值，应被拒绝", gt)
		}
	}
}

// TestLoginUnsupportedGrantType 客户端不支持的授权类型（sms 未在 grant_type 里）。
func TestLoginUnsupportedGrantType(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Login(context.Background(), &authmodel.LoginBody{
		ClientID:  seedClientID,
		GrantType: "sms",
		Username:  "admin",
		Password:  seedPassword,
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("客户端不支持 sms，应失败")
	}
	if !strings.Contains(err.Error(), "认证权限类型") {
		t.Errorf("提示应是认证权限类型错误: %v", err)
	}
}

// TestLoginUnknownClientFoldsIntoSameMessage 未知 clientId 与「授权类型不支持」
// 折叠成同一句提示。
//
// 对齐 Java（AuthController.java:85-88 两种情况回的都是 auth.grant.type.error）。
// 不区分也更安全 —— 否则这个接口就成了枚举有效 clientId 的工具。
func TestLoginUnknownClientFoldsIntoSameMessage(t *testing.T) {
	f := newFixture(t)

	_, unknownErr := f.svc.Login(context.Background(), &authmodel.LoginBody{
		ClientID:  "deadbeefdeadbeefdeadbeefdeadbeef",
		GrantType: "password",
		Username:  "admin",
		Password:  seedPassword,
	}, "127.0.0.1")
	if unknownErr == nil {
		t.Fatal("未知 clientId 应失败")
	}

	_, unsupportedErr := f.svc.Login(context.Background(), &authmodel.LoginBody{
		ClientID:  seedClientID,
		GrantType: "sms",
		Username:  "admin",
		Password:  seedPassword,
	}, "127.0.0.1")
	if unsupportedErr == nil {
		t.Fatal("不支持的授权类型应失败")
	}

	if unknownErr.Error() != unsupportedErr.Error() {
		t.Errorf("两种失败的文案应一致(避免枚举 clientId):\n未知客户端: %v\n类型不支持: %v",
			unknownErr, unsupportedErr)
	}
}

// TestLoginDisabledClient 停用的客户端不能用于登录。
func TestLoginDisabledClient(t *testing.T) {
	f := newFixture(t)

	if err := f.db.Model(&systemmodel.SysClient{}).
		Where("client_id = ?", seedClientID).
		Update("status", constant.StatusDisable).Error; err != nil {
		t.Fatalf("更新客户端状态失败: %v", err)
	}

	_, err := f.login("admin", seedPassword)
	if err == nil {
		t.Fatal("停用的客户端应登录失败")
	}
	if !strings.Contains(err.Error(), "禁用") {
		t.Errorf("提示应指明类型已禁用: %v", err)
	}
}

// TestLoginFreezesClientRulesIntoToken 客户端访问规则在签发时冻结进 token。
//
// 对齐 Java（SecurityConfig 从 token extra 读这三项，不查库）。
// 代价是改了客户端规则要等存量 token 过期才生效 —— 这是有意保持的行为，
// 否则鉴权热路径每请求都要查一次 sys_client。
func TestLoginFreezesClientRulesIntoToken(t *testing.T) {
	f := newFixture(t)

	if err := f.db.Model(&systemmodel.SysClient{}).
		Where("client_id = ?", seedClientID).
		Updates(map[string]any{
			"access_path":  "/app/**",
			"ip_whitelist": "10.0.0.0/8",
		}).Error; err != nil {
		t.Fatalf("更新客户端规则失败: %v", err)
	}

	vo, err := f.login("admin", seedPassword)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	claims, err := auth.Verify(vo.AccessToken, "test-secret")
	if err != nil {
		t.Fatalf("验签失败: %v", err)
	}

	if claims.ClientAccessPath != "/app/**" {
		t.Errorf("clientAccessPath = %q, 期望 /app/**", claims.ClientAccessPath)
	}
	if claims.ClientIPWhitelist != "10.0.0.0/8" {
		t.Errorf("clientIpWhitelist = %q, 期望 10.0.0.0/8", claims.ClientIPWhitelist)
	}
}

// TestLogoutRevokesSession 登出删会话，这是 JWT 唯一的作废手段。
func TestLogoutRevokesSession(t *testing.T) {
	f := newFixture(t)

	vo, err := f.login("admin", seedPassword)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if _, err := f.mr.Get(auth.TokenKeyPrefix + vo.AccessToken); err != nil {
		t.Fatalf("前提不成立: 会话应已写入")
	}

	if err := f.svc.Logout(context.Background(), vo.AccessToken); err != nil {
		t.Fatalf("登出失败: %v", err)
	}

	if _, err := f.mr.Get(auth.TokenKeyPrefix + vo.AccessToken); err == nil {
		t.Error("登出后会话应被删除")
	}
	// 在线记录也要清，对应 UserActionListener.doLogout。
	if _, err := f.mr.Get(constant.OnlineTokenKeyPrefix + vo.AccessToken); err == nil {
		t.Error("登出后在线用户记录应被清除")
	}
}

// TestLogoutIsIdempotent 空 token、未知 token、重复登出都不报错。
//
// 对齐 Java 那两个 catch (NotLoginException ignored)：登出失败没有任何
// 可行的补救动作，回错误只会让前端不敢清本地状态，
// 反而卡在一个「以为还登录着」的界面上。
func TestLogoutIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if err := f.svc.Logout(ctx, ""); err != nil {
		t.Errorf("空 token 登出不该报错: %v", err)
	}
	if err := f.svc.Logout(ctx, "never-issued-token"); err != nil {
		t.Errorf("未知 token 登出不该报错: %v", err)
	}

	vo, err := f.login("admin", seedPassword)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if err := f.svc.Logout(ctx, vo.AccessToken); err != nil {
		t.Fatalf("首次登出失败: %v", err)
	}
	if err := f.svc.Logout(ctx, vo.AccessToken); err != nil {
		t.Errorf("重复登出不该报错: %v", err)
	}
}

// TestLoginMalformedHashIsNotCountedAsWrongPassword 哈希串格式非法是**数据问题**，
// 不该计入密码错误次数 —— 否则会掩盖真正的原因（迁移时漏加密的明文密码）。
func TestLoginMalformedHashIsNotCountedAsWrongPassword(t *testing.T) {
	f := newFixture(t)

	// 模拟迁移时漏加密：库里存的是明文。
	if err := f.db.Model(&systemmodel.SysUser{}).
		Where("user_id = ?", seedUserID).
		Update("password", "plaintext-password").Error; err != nil {
		t.Fatalf("更新密码失败: %v", err)
	}

	if _, err := f.login("admin", "plaintext-password"); err == nil {
		t.Fatal("非法哈希不该校验通过")
	}
	if got := f.errCount(t, "admin"); got != -1 {
		t.Errorf("哈希格式非法不该计入密码错误次数, 得到 %d", got)
	}
}
