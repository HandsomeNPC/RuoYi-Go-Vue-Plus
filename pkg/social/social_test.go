package social

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"ruoyi-go-vue-plus/pkg/config"
)

// authorizeQuery 用给定配置拼出授权地址并解析出 query 参数。
func authorizeQuery(t *testing.T, p Provider, cfg config.SocialLoginConfig) url.Values {
	t.Helper()
	oc, err := p.OAuth2Config(cfg)
	if err != nil {
		t.Fatalf("OAuth2Config 失败: %v", err)
	}
	u, err := url.Parse(oc.AuthCodeURL("st-123"))
	if err != nil {
		t.Fatalf("解析授权地址失败: %v", err)
	}
	return u.Query()
}

// TestAuthorizeURLCarriesOAuth2Params 授权地址须带齐 OAuth2 四件套。
// 这四个参数由 x/oauth2 的 AuthCodeURL 负责，钉住它是为了防端点/配置串错位
// (如把 RedirectURI 填进 ClientID)——那样拼出的地址一样合法，只是永远换不到令牌。
func TestAuthorizeURLCarriesOAuth2Params(t *testing.T) {
	cfg := config.SocialLoginConfig{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/social-callback?source=gitee",
	}
	q := authorizeQuery(t, giteeProvider{}, cfg)

	for key, want := range map[string]string{
		"response_type": "code",
		"client_id":     "cid",
		"redirect_uri":  cfg.RedirectURI,
		"state":         "st-123",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// TestDefaultScopes gitee 有默认 scope、github 没有。
//
// 差异源自 JustAuth：AuthGiteeScope 里 user_info 标了 isDefault=true，
// 而 AuthGithubScope 全部 isDefault=false，getScopes 因而返回空、地址不带 scope。
// 值得钉：给 github 硬塞一个 scope 会让授权页多要权限，用户侧可感知。
func TestDefaultScopes(t *testing.T) {
	cfg := config.SocialLoginConfig{ClientID: "cid", ClientSecret: "s", RedirectURI: "http://x/cb"}

	if got := authorizeQuery(t, giteeProvider{}, cfg).Get("scope"); got != giteeDefaultScope {
		t.Errorf("gitee scope = %q, want %q", got, giteeDefaultScope)
	}
	if q := authorizeQuery(t, githubProvider{}, cfg); q.Has("scope") {
		t.Errorf("github 不应带 scope, 实际 %q", q.Get("scope"))
	}
}

// TestConfiguredScopesOverrideDefault 配置里显式给了 scopes 就不用默认值。
func TestConfiguredScopesOverrideDefault(t *testing.T) {
	cfg := config.SocialLoginConfig{
		ClientID: "cid", ClientSecret: "s", RedirectURI: "http://x/cb",
		Scopes: []string{"projects", "issues"},
	}
	// x/oauth2 用空格连接多个 scope。
	if got, want := authorizeQuery(t, giteeProvider{}, cfg).Get("scope"), "projects issues"; got != want {
		t.Errorf("scope = %q, want %q", got, want)
	}
}

// TestServerProviderEndpoints maxkey/topiam 的端点由 serverUrl 套模板拼出。
func TestServerProviderEndpoints(t *testing.T) {
	cases := []struct {
		source    string
		serverURL string
		wantAuth  string
		wantToken string
		wantStyle oauth2.AuthStyle
	}{
		{
			source: SourceMaxkey, serverURL: "http://sso.maxkey.top",
			wantAuth:  "http://sso.maxkey.top/sign/authz/oauth/v20/authorize",
			wantToken: "http://sso.maxkey.top/sign/authz/oauth/v20/token",
			wantStyle: oauth2.AuthStyleInParams,
		},
		{
			source: SourceTopiam, serverURL: "https://iam.example.com",
			wantAuth:  "https://iam.example.com/oauth2/auth",
			wantToken: "https://iam.example.com/oauth2/token",
			// topiam 的 token 端点走 HTTP Basic，与 maxkey 的参数体形态不同。
			wantStyle: oauth2.AuthStyleInHeader,
		},
	}
	for _, c := range cases {
		t.Run(c.source, func(t *testing.T) {
			oc, err := serverProvider{source: c.source}.OAuth2Config(config.SocialLoginConfig{
				ClientID: "cid", ClientSecret: "s", RedirectURI: "http://x/cb", ServerURL: c.serverURL,
			})
			if err != nil {
				t.Fatalf("OAuth2Config 失败: %v", err)
			}
			if oc.Endpoint.AuthURL != c.wantAuth {
				t.Errorf("AuthURL = %q, want %q", oc.Endpoint.AuthURL, c.wantAuth)
			}
			if oc.Endpoint.TokenURL != c.wantToken {
				t.Errorf("TokenURL = %q, want %q", oc.Endpoint.TokenURL, c.wantToken)
			}
			if oc.Endpoint.AuthStyle != c.wantStyle {
				t.Errorf("AuthStyle = %v, want %v", oc.Endpoint.AuthStyle, c.wantStyle)
			}
		})
	}
}

// TestNormalizeServerURLTrimsTrailingSlash 尾部斜杠必须剥掉。
// 不剥的话 "%s/oauth2/auth" 会拼出 "//oauth2/auth"，多数网关直接 404。
func TestNormalizeServerURLTrimsTrailingSlash(t *testing.T) {
	for _, in := range []string{"http://x.com", "http://x.com/", "http://x.com///"} {
		got, err := normalizeServerURL(in)
		if err != nil {
			t.Fatalf("normalizeServerURL(%q) 报错: %v", in, err)
		}
		if got != "http://x.com" {
			t.Errorf("normalizeServerURL(%q) = %q, want %q", in, got, "http://x.com")
		}
	}
}

// TestNormalizeServerURLRejectsInvalid 非法 serverUrl 要在注册期就被挡下。
func TestNormalizeServerURLRejectsInvalid(t *testing.T) {
	for _, in := range []string{
		"",                  // 未配置
		"sso.maxkey.top",    // 缺协议
		"ftp://x.com",       // 协议不是 http/https
		"http://",           // 缺主机名
		"http://x.com?a=1",  // 带查询串
		"http://x.com#frag", // 带片段
	} {
		if _, err := normalizeServerURL(in); err == nil {
			t.Errorf("normalizeServerURL(%q) 应报错，实际通过", in)
		}
	}
}

// TestServerProviderRequiresServerURL 私有化部署平台缺 serverUrl 时构造即失败，
// 这正是 Init 用来把它剔出注册表的信号。
func TestServerProviderRequiresServerURL(t *testing.T) {
	_, err := serverProvider{source: SourceTopiam}.OAuth2Config(config.SocialLoginConfig{
		ClientID: "cid", ClientSecret: "s", RedirectURI: "http://x/cb",
	})
	if err == nil {
		t.Fatal("缺 serverUrl 应报错，实际通过")
	}
}

// TestTokenFrom oauth2.Token 摊平成 AuthToken：
// scope/id_token 得从 Extra 里取，有效期要从时间点换算成秒数。
func TestTokenFrom(t *testing.T) {
	tok := (&oauth2.Token{
		AccessToken:  "at",
		RefreshToken: "rt",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}).WithExtra(map[string]any{"scope": "user_info", "id_token": "idt"})

	got := tokenFrom(tok)
	if got.AccessToken != "at" || got.RefreshToken != "rt" || got.TokenType != "Bearer" {
		t.Errorf("基本字段未透传: %+v", got)
	}
	if got.Scope != "user_info" || got.IDToken != "idt" {
		t.Errorf("Extra 字段未取到: scope=%q idToken=%q", got.Scope, got.IDToken)
	}
	// 一小时约 3600 秒，容忍执行耗时导致的几秒偏差。
	if got.ExpireIn < 3590 || got.ExpireIn > 3600 {
		t.Errorf("ExpireIn = %d, want ≈3600", got.ExpireIn)
	}
}

// TestTokenFromNoExpiry 平台不给有效期时(如 github) ExpireIn 留 0，
// 不能算成负数——那会被落库成一个荒谬的过期时间。
func TestTokenFromNoExpiry(t *testing.T) {
	if got := tokenFrom(&oauth2.Token{AccessToken: "at"}); got.ExpireIn != 0 {
		t.Errorf("ExpireIn = %d, want 0", got.ExpireIn)
	}
}

// TestLookupUnregisteredSource 未注册的平台一律 ErrUnsupportedSource，
// handler 靠它转成「xx平台账号暂不支持」。
// 前端那个 wechat 按钮走的就是这条路径：它不是合法 source，原项目同样报不支持。
func TestLookupUnregisteredSource(t *testing.T) {
	mu.Lock()
	registry = map[string]Provider{}
	mu.Unlock()

	for _, source := range []string{"wechat", "gitee", "不存在的平台"} {
		if _, _, err := lookup(source); err != ErrUnsupportedSource {
			t.Errorf("lookup(%q) err = %v, want ErrUnsupportedSource", source, err)
		}
	}
}

// TestFirstNonEmpty maxkey 的 uuid 要在 userId/userid/sub 三个键里挑第一个非空的。
func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "sub-1"); got != "sub-1" {
		t.Errorf("firstNonEmpty = %q, want %q", got, "sub-1")
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("全空应返回空串, 实际 %q", got)
	}
}

// TestStateKeyPrefix state 的 Redis 键须落在固定前缀下，
// 否则同库共存的不同进程会各写一处、互不认账。
func TestStateKeyPrefix(t *testing.T) {
	if got := stateKey("abc"); !strings.HasPrefix(got, "global:social_auth_codes:") {
		t.Errorf("stateKey = %q, 前缀不符", got)
	}
}

// TestCheckStateRejectsEmpty 空 state 不必查 Redis 就该拒——
// 这条单测不依赖 Redis，故与其余 state 行为(需真实 Redis)分开放。
func TestCheckStateRejectsEmpty(t *testing.T) {
	if err := checkState(nil, ""); err != ErrIllegalState {
		t.Errorf("checkState(\"\") = %v, want ErrIllegalState", err)
	}
}
