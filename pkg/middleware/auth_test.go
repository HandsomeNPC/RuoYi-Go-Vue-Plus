package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/auth"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/response"
)

const (
	authTestSecret   = "test-secret"
	authTestClientID = "e5cd7e4891bf95d1d19206ce24a7b32e" // 种子数据里的 pc 客户端
)

// authFixture 一套自洽的鉴权测试环境：内存 Redis + 引擎 + 已签发的会话。
type authFixture struct {
	engine *gin.Engine
	mr     *miniredis.Miniredis
	store  *auth.SessionStore
}

// authCfg 以真实默认配置为基准，再按需覆盖。
//
// **不要直接传 config.Auth{}**：那份零值的 TokenPrefix 是空串，而空串在本
// 中间件里是**有意义的取值**（不使用前缀），于是 "Bearer xxx" 不会被剥掉、
// 每个请求都 401。正常进程走 viper 铺的默认值，测试也应从同一处取，
// 否则测的是一份现实中不存在的配置。
func authCfg(override func(*config.Auth)) config.Auth {
	cfg := config.DefaultMiddleware().Auth
	if override != nil {
		override(&cfg)
	}
	return cfg
}

// newAuthFixture 构造只挂 Recover + Auth 的引擎。
//
// **必须带上 Recover**：本中间件的错误走 c.Error 而非自己写响应体，
// 由 Recover 统一渲染成 response.R。不挂它就测不到真实的响应形态 ——
// 那也正是 TestAuthErrorsRenderedAsHTTP200 要锁的东西。
func newAuthFixture(t *testing.T, cfg config.Auth) *authFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	r := gin.New()
	r.Use(Recover())
	r.Use(AuthWithConfig(cfg, authTestSecret, rdb))

	// 受保护接口：回写中间件写进两处上下文的用户，确认它们一致。
	handler := func(c *gin.Context) {
		var fromCtx string
		if u := UserFromContext(c.Request.Context()); u != nil {
			fromCtx = u.Username
		}
		var fromGin string
		if u := CurrentUser(c); u != nil {
			fromGin = u.Username
		}
		c.JSON(http.StatusOK, gin.H{"fromGin": fromGin, "fromCtx": fromCtx})
	}
	r.GET("/system/user/list", handler)
	r.GET("/app/profile", handler)
	r.POST("/auth/login", handler)
	r.GET("/index.html", handler)

	return &authFixture{engine: r, mr: mr, store: auth.NewSessionStore(rdb)}
}

// issue 签发一个 token 并写入对应会话，返回裸 token。
func (f *authFixture) issue(t *testing.T, claims *auth.Claims, activeTimeout int64) string {
	t.Helper()

	if claims.ClientID == "" {
		claims.ClientID = authTestClientID
	}
	token, err := auth.Sign(claims, authTestSecret, time.Hour)
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	sess := &auth.Session{
		User: &auth.LoginUser{
			UserID:   claims.UserID,
			Username: claims.Username,
			UserType: auth.UserTypeSys,
		},
		ActiveTimeout: activeTimeout,
	}
	if err := f.store.Save(context.Background(), token, sess); err != nil {
		t.Fatalf("写入会话失败: %v", err)
	}
	return token
}

// do 发一次请求。token 为空表示不带 Authorization 头。
func (f *authFixture) do(method, path, token, clientID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set(config.TokenHeader, config.TokenPrefix+" "+token)
	}
	if clientID != "" {
		req.Header.Set(config.ClientIDHeader, clientID)
	}
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	return w
}

// bodyCode 取响应体里的业务码。
func bodyCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("解析响应体失败: %v (body=%q)", err, w.Body.String())
	}
	return r.Code
}

// bodyMsg 取响应体里的提示文案。
func bodyMsg(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var r struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("解析响应体失败: %v (body=%q)", err, w.Body.String())
	}
	return r.Msg
}

// 正常路径：带有效 token 与匹配的 clientid 应放行，
// 且登录用户同时出现在 gin.Context 与 request context 里。
//
// 两处都断言是刻意的：handler 走 gin.Context，而 service / repository 层走
// request context（阶段 4.1 的数据权限要用）。只写一处会让另一条路径
// 在真正需要时才发现是空的。
func TestAuthAllowsValidToken(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1761100000000000001, Username: "admin"}, 1800)

	w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		FromGin string `json:"fromGin"`
		FromCtx string `json:"fromCtx"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got.FromGin != "admin" {
		t.Errorf("gin.Context 里的用户 = %q, 期望 admin", got.FromGin)
	}
	if got.FromCtx != "admin" {
		t.Errorf("request context 里的用户 = %q, 期望 admin", got.FromCtx)
	}
}

// TestAuthErrorsRenderedAsHTTP200 锁住「HTTP 状态码恒 200，业务码放响应体」。
//
// 这是 recover.go 已有的硬约束：Java 侧那些 advice 返回 R<Void> 且未标
// @ResponseStatus，前端统一拦 body.code 判成败。改成真实 401/403
// 会让前端拦截器失效 —— 而那种故障表现为「所有接口都报错」，
// 却与本次改动看不出关系。
func TestAuthErrorsRenderedAsHTTP200(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	w := f.do(http.MethodGet, "/system/user/list", "", "")
	if w.Code != http.StatusOK {
		t.Errorf("HTTP 状态码 = %d, 必须恒为 200(业务码放响应体)", w.Code)
	}
	if got := bodyCode(t, w); got != response.CodeUnauthorized {
		t.Errorf("响应体 code = %d, 期望 %d", got, response.CodeUnauthorized)
	}
	if got := bodyMsg(t, w); got != msgNotLogin {
		t.Errorf("响应体 msg = %q, 期望 %q", got, msgNotLogin)
	}
}

// 无 token / 畸形 token / 异密钥签发的 token 一律 401。
func TestAuthRejectsMissingOrBadToken(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	forged, err := auth.Sign(&auth.Claims{UserID: 1, ClientID: authTestClientID},
		"wrong-secret", time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}

	tests := []struct{ name, token string }{
		{"无 token", ""},
		{"畸形 token", "not-a-jwt"},
		{"异密钥签发", forged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := f.do(http.MethodGet, "/system/user/list", tt.token, authTestClientID)
			if got := bodyCode(t, w); got != response.CodeUnauthorized {
				t.Errorf("code = %d, 期望 401", got)
			}
		})
	}
}

// TestAuthRejectsAlgNone 锁住 pkg/auth.Verify 传了 jwt.WithValidMethods。
//
// 中间件层面再测一遍（pkg/auth 已有同名用例）：这条链路是完整的鉴权绕过，
// 值得在两层各留一道锁 —— 将来谁重构 Verify 的调用方式，中间件这条也会红。
func TestAuthRejectsAlgNone(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	// header/payload 手工拼一个 alg=none 的 token，签名段留空。
	const unsigned = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJ1c2VySWQiOjEsImNsaWVudGlkIjoiZTVjZDdlNDg5MWJmOTVkMWQxOTIwNmNlMjRhN2IzMmUifQ."

	w := f.do(http.MethodGet, "/system/user/list", unsigned, authTestClientID)
	if got := bodyCode(t, w); got != response.CodeUnauthorized {
		t.Errorf("alg=none 必须被拒: code = %d, 期望 401", got)
	}
}

// 过期 token 的文案与「非法」不同 —— 两者前端行为相同，但日志里必须能区分
// 「正常用到过期」和「token 被篡改」。
func TestAuthExpiredTokenHasOwnMessage(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	expired, err := auth.Sign(&auth.Claims{UserID: 1, ClientID: authTestClientID},
		authTestSecret, -time.Minute)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}

	w := f.do(http.MethodGet, "/system/user/list", expired, authTestClientID)
	if got := bodyMsg(t, w); got != msgTokenExpired {
		t.Errorf("msg = %q, 期望 %q", got, msgTokenExpired)
	}
}

// TestAuthRequiresSession 只验签不查会话，登出就形同虚设。
//
// token 本身完全有效（签名对、未过期），但会话已被删除 —— 必须 401。
// 这条锁住 JWT 的撤销机制。
func TestAuthRequiresSession(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	// 先确认能通。
	if w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID); w.Code != http.StatusOK {
		t.Fatalf("前提不成立: 有效 token 应放行, body=%s", w.Body.String())
	}

	// 模拟登出。
	if err := f.store.Delete(context.Background(), token); err != nil {
		t.Fatalf("删除会话失败: %v", err)
	}

	w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID)
	if got := bodyCode(t, w); got != response.CodeUnauthorized {
		t.Errorf("会话已删除后必须 401: code = %d", got)
	}
	if got := bodyMsg(t, w); got != msgTokenExpired {
		t.Errorf("msg = %q, 期望 %q", got, msgTokenExpired)
	}
}

// clientid 不匹配返 401，对应 Java 抛 NotLoginException code -100。
func TestAuthClientIDMismatch(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	tests := []struct{ name, clientID string }{
		{"完全不同", "428a8310cd442757ae699df5d894f051"},
		{"未携带", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := f.do(http.MethodGet, "/system/user/list", token, tt.clientID)
			if got := bodyCode(t, w); got != response.CodeUnauthorized {
				t.Errorf("code = %d, 期望 401", got)
			}
		})
	}
}

// TestAuthClientIDFromQueryAlsoWorks header 或 query 命中其一即可，
// 对应 Java 的 StringUtils.equalsAny(clientId, headerCid, paramCid)。
func TestAuthClientIDFromQueryAlsoWorks(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	req := httptest.NewRequest(http.MethodGet,
		"/system/user/list?"+config.ClientIDHeader+"="+authTestClientID, nil)
	req.Header.Set(config.TokenHeader, config.TokenPrefix+" "+token)
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("查询串里的 clientid 应生效: body=%s", w.Body.String())
	}
}

// TestAuthMissingClientIDClaimDoesNotPanic 锁住相对 Java 的一处有意偏差。
//
// Java 侧 StpUtil.getExtra(CLIENT_KEY).toString()（SecurityConfig.java:100）
// 对缺 clientid claim 的 token 会 **NPE 成 500**，那是 bug 不是可对齐的行为。
// Go 侧视为不匹配返 401。
//
// 关键点：请求也不带 clientid 时，两个空串**不能**被判成"相等"从而放行 ——
// 那会让一个精心构造的 token 绕过整个客户端绑定。
func TestAuthMissingClientIDClaimDoesNotPanic(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	// 显式签一个不含 clientid 的 token（绕开 issue 的默认填充）。
	token, err := auth.Sign(&auth.Claims{UserID: 1, Username: "admin"},
		authTestSecret, time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	sess := &auth.Session{
		User:          &auth.LoginUser{UserID: 1, Username: "admin", UserType: auth.UserTypeSys},
		ActiveTimeout: 1800,
	}
	if err := f.store.Save(context.Background(), token, sess); err != nil {
		t.Fatalf("写入会话失败: %v", err)
	}

	// 请求同样不带 clientid：两个空串不能判等。
	w := f.do(http.MethodGet, "/system/user/list", token, "")
	if w.Code != http.StatusOK {
		t.Errorf("不该是 500(Java 侧的 NPE), HTTP 状态码 = %d", w.Code)
	}
	if got := bodyCode(t, w); got != response.CodeUnauthorized {
		t.Errorf("缺 clientid claim 应返 401: code = %d", got)
	}
}

// TestAuthTokenNotReadFromQueryOrCookie 锁住「token 只从 header 取」。
//
// sa-token 配了 is-read-body: true（可从请求参数读），Go 侧**有意不跟**：
// 查询串会流进 nginx accesslog 与浏览器历史；cookie 则是原项目
// is-read-cookie: false 刻意关掉以杜绝 CSRF 的（README 已写明
// 「不要为了方便加 cookie 回落」）。
//
// clientid 仍保留 header-or-query —— 那个不是凭证、只是标识。
func TestAuthTokenNotReadFromQueryOrCookie(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	t.Run("查询串", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/system/user/list?"+config.TokenHeader+"="+token+
				"&"+config.ClientIDHeader+"="+authTestClientID, nil)
		w := httptest.NewRecorder()
		f.engine.ServeHTTP(w, req)
		if got := bodyCode(t, w); got != response.CodeUnauthorized {
			t.Errorf("查询串里的 token 不该生效: code = %d", got)
		}
	})

	t.Run("cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/system/user/list", nil)
		req.AddCookie(&http.Cookie{Name: config.TokenHeader, Value: token})
		req.Header.Set(config.ClientIDHeader, authTestClientID)
		w := httptest.NewRecorder()
		f.engine.ServeHTTP(w, req)
		if got := bodyCode(t, w); got != response.CodeUnauthorized {
			t.Errorf("cookie 里的 token 不该生效(CSRF): code = %d", got)
		}
	})
}

// TestAuthSkipsExcludes 免鉴权名单里的路径不需要 token。
func TestAuthSkipsExcludes(t *testing.T) {
	f := newAuthFixture(t, authCfg(func(c *config.Auth) {
		c.Excludes = []string{"/auth/**", "/**/*.html"}
	}))

	for _, path := range []string{"/auth/login", "/index.html"} {
		method := http.MethodGet
		if path == "/auth/login" {
			method = http.MethodPost
		}
		if w := f.do(method, path, "", ""); w.Code != http.StatusOK {
			t.Errorf("%s 在免鉴权名单里，不带 token 也应放行: 状态码 = %d, body=%s",
				path, w.Code, w.Body.String())
		}
	}
}

// TestAuthEmptyExcludesProtectsEverything 空名单意味着「什么都不排除」，
// 不是「全部排除」—— 这个默认值取反就是敞开的口子。
func TestAuthEmptyExcludesProtectsEverything(t *testing.T) {
	f := newAuthFixture(t, authCfg(func(c *config.Auth) { c.Excludes = nil }))

	if got := bodyCode(t, f.do(http.MethodPost, "/auth/login", "", "")); got != response.CodeUnauthorized {
		t.Errorf("空 excludes 时连 /auth/login 也应鉴权: code = %d", got)
	}
}

// TestAuthUnregisteredPathFalls404NotUnauthorized 锁住对齐 Java AllUrlHandler 的行为。
//
// Java 侧 SaInterceptor 内部先 SaRouter.match(allUrlHandler.getUrls()) 筛一道，
// 那是启动时收集的**全部已注册路由**。未注册的路径根本不进鉴权，
// 直接落 404 而非 401。Go 侧用 c.FullPath() == "" 判定同一件事。
//
// 前端与测试可能依赖「乱路径返 404」这个行为，故显式锁住。
func TestAuthUnregisteredPathFalls404NotUnauthorized(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))

	// 不带 token 访问未注册路径。
	w := f.do(http.MethodGet, "/nonexistent/path", "", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("未注册路径应落 404(不进鉴权): 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
}

// TestAuthAccessPathReturns403 客户端访问路径白名单，
// 对应 NotPermissionException("当前客户端未授权访问该接口路径")。
//
// 种子数据里 app 客户端的 access_path 是 /app/**，即它签发的 token
// 不能访问 /system/** —— 这正是该机制存在的理由。
func TestAuthAccessPathReturns403(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{
		UserID:           1,
		Username:         "appuser",
		ClientAccessPath: "/app/**",
	}, 1800)

	// 命中白名单 -> 放行。
	if w := f.do(http.MethodGet, "/app/profile", token, authTestClientID); w.Code != http.StatusOK {
		t.Errorf("/app/profile 在白名单内应放行: body=%s", w.Body.String())
	}

	// 未命中 -> 403（不是 401：身份是清楚的，只是不允许访问）。
	w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID)
	if got := bodyCode(t, w); got != response.CodeForbidden {
		t.Errorf("越界访问应返 403: code = %d", got)
	}
	if got := bodyMsg(t, w); got != msgNoPermission {
		t.Errorf("msg = %q, 期望 %q", got, msgNoPermission)
	}
}

// TestAuthEmptyAccessPathMeansUnrestricted access_path 为空即不限制。
//
// 对齐 Java 的 StringUtils.isNotBlank 判断 —— sys_client 允许该列为 null，
// 种子数据的 pc 客户端就是空的。若把空当成「什么都不许」，pc 端全挂。
func TestAuthEmptyAccessPathMeansUnrestricted(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin", ClientAccessPath: ""}, 1800)

	if w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID); w.Code != http.StatusOK {
		t.Errorf("access_path 为空应不受限: body=%s", w.Body.String())
	}
}

// TestAuthIPWhitelistReturns403 客户端 IP 白名单，
// 对应 NotPermissionException("当前客户端IP不在白名单内")。
func TestAuthIPWhitelistReturns403(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{
		UserID:            1,
		Username:          "admin",
		ClientIPWhitelist: "10.0.0.0/8, 192.168.1.*",
	}, 1800)

	newReq := func(clientIP string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/system/user/list", nil)
		req.Header.Set(config.TokenHeader, config.TokenPrefix+" "+token)
		req.Header.Set(config.ClientIDHeader, authTestClientID)
		req.Header.Set("X-Forwarded-For", clientIP)
		w := httptest.NewRecorder()
		f.engine.ServeHTTP(w, req)
		return w
	}

	// 命中 CIDR 与 glob 两种规则都应放行。
	for _, ip := range []string{"10.1.2.3", "192.168.1.99"} {
		if w := newReq(ip); w.Code != http.StatusOK {
			t.Errorf("IP %s 在白名单内应放行: body=%s", ip, w.Body.String())
		}
	}

	// 白名单外 -> 403。
	w := newReq("8.8.8.8")
	if got := bodyCode(t, w); got != response.CodeForbidden {
		t.Errorf("白名单外的 IP 应返 403: code = %d", got)
	}
}

// TestAuthRenewsSessionTTL 校验通过后滑动续期。
//
// 锁住「滑动」而非「固定」窗口：活跃用户不该在登录满 activeTimeout 后被登出。
// 这是原项目 dynamic-active-timeout: true 的效果。
func TestAuthRenewsSessionTTL(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	// 消耗掉一半空闲窗口。
	f.mr.FastForward(900 * time.Second)
	if got := f.mr.TTL(auth.TokenKeyPrefix + token); got != 900*time.Second {
		t.Fatalf("前提不成立: 推进后 TTL = %v, 期望 900s", got)
	}

	if w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID); w.Code != http.StatusOK {
		t.Fatalf("请求应通过: body=%s", w.Body.String())
	}

	if got, want := f.mr.TTL(auth.TokenKeyPrefix+token), 1800*time.Second; got != want {
		t.Errorf("请求后 TTL 应滑动重置为 %v, 得到 %v", want, got)
	}
}

// TestAuthAcceptsBareTokenWhenNoPrefix TokenPrefix 配空串表示不使用前缀。
//
// 那是**有意义的取值**而非「用默认值」，故不回落默认值。
func TestAuthAcceptsBareTokenWhenNoPrefix(t *testing.T) {
	f := newAuthFixture(t, authCfg(func(c *config.Auth) { c.TokenPrefix = "" }))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	req := httptest.NewRequest(http.MethodGet, "/system/user/list", nil)
	req.Header.Set(config.TokenHeader, token) // 裸 token，无 Bearer
	req.Header.Set(config.ClientIDHeader, authTestClientID)
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("不配前缀时裸 token 应生效: body=%s", w.Body.String())
	}
}

// TestAuthRedisFailureIsNotUnauthorized Redis 故障必须与「未登录」区分。
//
// 回 401 会让一次 Redis 抖动表现成「所有人被登出」，而日志里只有一片 401，
// 看不出真正的原因。应交给 Recover 兜成系统异常（500 业务码）并打日志。
func TestAuthRedisFailureIsNotUnauthorized(t *testing.T) {
	f := newAuthFixture(t, authCfg(nil))
	token := f.issue(t, &auth.Claims{UserID: 1, Username: "admin"}, 1800)

	f.mr.Close() // Redis 不可用

	w := f.do(http.MethodGet, "/system/user/list", token, authTestClientID)
	if got := bodyCode(t, w); got == response.CodeUnauthorized {
		t.Error("Redis 故障不该表现为 401(会被误读成「所有人被登出」)")
	}
	if got := bodyCode(t, w); got != response.CodeFail {
		t.Errorf("Redis 故障应兜成系统异常 500: code = %d", got)
	}
}

// TestNewUserContext 脱离请求的场景（定时任务、消息消费）能构造用户上下文。
func TestNewUserContext(t *testing.T) {
	user := &auth.LoginUser{UserID: 1, Username: "admin"}
	ctx := NewUserContext(context.Background(), user)

	if got := UserFromContext(ctx); got != user {
		t.Errorf("UserFromContext 应取回同一个用户, got %v", got)
	}
	// 未设置时返回 nil 而非 panic。
	if got := UserFromContext(context.Background()); got != nil {
		t.Errorf("空 context 应返回 nil, got %v", got)
	}
	if got := UserFromContext(nil); got != nil {
		t.Errorf("nil context 应返回 nil, got %v", got)
	}
	if got := CurrentUser(nil); got != nil {
		t.Errorf("nil gin.Context 应返回 nil, got %v", got)
	}
}

// TestSplitClientRules 规则串按 , ; CR LF 切分，trim 并丢弃空段。
//
// 对应 Java 的 CLIENT_RULE_SEPARATOR_REGEX "[,;\r\n]+" + str2List(…, true, true)。
func TestSplitClientRules(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"/app/**", []string{"/app/**"}},
		{"/app/**,/pub/**", []string{"/app/**", "/pub/**"}},
		{"/app/**;/pub/**", []string{"/app/**", "/pub/**"}},
		{"/app/**\r\n/pub/**", []string{"/app/**", "/pub/**"}},
		// 连续分隔符合并、空段丢弃（对应 "+" 量词与 ignoreEmpty）。
		{"/app/**,,;/pub/**", []string{"/app/**", "/pub/**"}},
		{" /app/** , /pub/** ", []string{"/app/**", "/pub/**"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		got := splitClientRules(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitClientRules(%q) = %v, 期望 %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitClientRules(%q) = %v, 期望 %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

// TestMatchesAnySkipsEmpty 空候选值必须跳过。
//
// 请求没带 clientid 时 header 与 query 都是空串，若不跳过，
// 一个 clientid 为空的 token 就能匹配上任何不带该头的请求。
func TestMatchesAnySkipsEmpty(t *testing.T) {
	if matchesAny("", "", "") {
		t.Error("空 want 与空候选值不该判等")
	}
	if matchesAny("abc", "", "") {
		t.Error("候选值全空时不该匹配")
	}
	if !matchesAny("abc", "", "abc") {
		t.Error("第二个候选值命中应匹配")
	}
	if !matchesAny("abc", "abc", "") {
		t.Error("第一个候选值命中应匹配")
	}
}
