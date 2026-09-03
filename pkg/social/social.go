// Package social 三方账号授权登录。
//
// Java 侧用 JustAuth 提供全部平台实现，Go 生态无对应物：
// golang.org/x/oauth2 只覆盖 OAuth2 协议本身(拼授权地址、code 换令牌)，
// 而「拿令牌去哪换用户资料、换回什么形状」不在 RFC 6749 范围内——
// oauth2.Endpoint 只有 AuthURL/TokenURL 两个地址字段，没有 userinfo。
// 故本包的分工是：
//
//	x/oauth2  负责 AuthCodeURL / Exchange / client 认证方式协商
//	Provider  负责端点 URL 表 + userinfo 请求 + 字段映射
//	state.go  负责 state 存取(Redis)
//
// 加一个平台 = 实现 Provider 的两个方法并在 Init 里注册，约 30 行。
//
// 初始化对照 sms.Init / mail.Init：social.Init() 无参，自读 config.Get().Social，
// 但依赖 Redis(state 存 Redis)，须排在 redis.Init 之后。
package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"ruoyi-go-vue-plus/pkg/config"
)

// 已接入的平台标识。取值即前端传来的 source，也是 sys_social.source 的落库值。
const (
	SourceGitee  = "gitee"
	SourceGithub = "github"
	SourceMaxkey = "maxkey"
	SourceTopiam = "topiam"
)

// ErrUnsupportedSource 平台未接入或配置不全。
// 由 handler 转成「xx平台账号暂不支持」，对齐 Java AuthController.authBinding
// 取不到 SocialLoginConfigProperties 时的 R.fail 分支。
var ErrUnsupportedSource = errors.New("social: 不支持的第三方登录类型")

// httpTimeout 单次请求超时。不设的话三方网关抽风会把 handler 协程挂死。
const httpTimeout = 10 * time.Second

// AuthUser 三方用户信息。
//
// 字段按 sys_social 的列取齐，不复用第三方库的用户结构：
// goth.User 没有 Scope/TokenType，且用 ExpiresAt time.Time 表达有效期，
// 而 sys_social.expire_in 是秒数。
type AuthUser struct {
	// UUID 平台内的用户唯一标识。落 open_id，并与 Source 拼成 auth_id。
	UUID     string
	Source   string
	Username string
	Nickname string
	Avatar   string
	Email    string
	Token    AuthToken
}

// AuthToken 令牌信息，字段与 sys_social 的令牌列一一对应。
// 多数平台只填得起前几项，其余留零(Java 侧 BeanUtil.copyProperties 亦然)。
type AuthToken struct {
	AccessToken  string
	ExpireIn     int
	RefreshToken string
	Scope        string
	TokenType    string
	IDToken      string
	OpenID       string
	UnionID      string
	AccessCode   string
	Code         string
}

// Provider 一个三方平台的接入点。
type Provider interface {
	// OAuth2Config 构造交给 x/oauth2 的配置：端点地址与默认 scope 由各平台定。
	// serverUrl 非法等配置问题在此返回错误。
	OAuth2Config(cfg config.SocialLoginConfig) (*oauth2.Config, error)

	// FetchUser 用令牌换用户资料。
	// RFC 6749 不覆盖这一步，故每个平台都得自己实现。
	FetchUser(ctx context.Context, cfg config.SocialLoginConfig,
		client *http.Client, tok *oauth2.Token) (*AuthUser, error)
}

// registry 已注册的平台。仅含配置齐全者，故「未注册」等价于「不支持」。
var (
	mu       sync.RWMutex
	registry = map[string]Provider{}
)

// providers 全部平台实现。与配置无关，是否可用由 Init 按配置筛。
var providers = map[string]Provider{
	SourceGitee:  giteeProvider{},
	SourceGithub: githubProvider{},
	SourceMaxkey: serverProvider{source: SourceMaxkey},
	SourceTopiam: serverProvider{source: SourceTopiam},
}

// Init 按 config.Get().Social 注册配置齐全的平台。须在 config.Load 之后调用。
//
// 配置不全即不注册，等价于 Java AuthChecker.isSupportedAuth——
// 它把缺 clientId/clientSecret(私有化部署再加 serverUrl)也归进「不支持该平台」。
// 因此前端那个 wechat 按钮天然返回「暂不支持」：它不是合法 source，
// 原项目 justauth 段里也只有 wechat_open/wechat_mp/wechat_enterprise。
func Init() {
	c := config.Get()

	reg := make(map[string]Provider, len(providers))
	for source, p := range providers {
		lc, ok := c.Social.Type[source]
		if !ok || !lc.Configured() {
			continue
		}
		// 让 provider 自查其余必填项(如私有化部署的 serverUrl)，
		// 不合格的当场剔除，免得留到运行期才报错。
		if _, err := p.OAuth2Config(lc); err != nil {
			log.Printf("[%s] social 平台 %s 配置无效，已跳过: %v", c.Server.Name, source, err)
			continue
		}
		reg[source] = p
	}

	mu.Lock()
	registry = reg
	mu.Unlock()

	log.Printf("[%s] social 已就绪: 可用平台=%v", c.Server.Name, sortedKeys(reg))
}

// lookup 取平台实现与其配置，未注册时返回 ErrUnsupportedSource。
func lookup(source string) (Provider, config.SocialLoginConfig, error) {
	mu.RLock()
	p, ok := registry[source]
	mu.RUnlock()
	if !ok {
		return nil, config.SocialLoginConfig{}, ErrUnsupportedSource
	}
	return p, config.Get().Social.Type[source], nil
}

// GetAuthorizeURL 生成授权跳转地址，并把 state 存进 Redis 待回调校验。
//
// 对齐 Java authRequest.authorize(AuthStateUtils.createState())：
// state 是裸 UUID。前端(RuoYi-Plus-UI 的 SocialCallback/index.vue)把它原样回传，
// 不做 base64/JSON 解码——RuoYi-Cloud-Plus 那套 Base64(JSON{domain,state})
// 是配 vben5 前端的，此处不可混用。
func GetAuthorizeURL(ctx context.Context, source string) (string, error) {
	p, lc, err := lookup(source)
	if err != nil {
		return "", err
	}
	oc, err := p.OAuth2Config(lc)
	if err != nil {
		return "", err
	}

	state := newState()
	if err := cacheState(ctx, state); err != nil {
		return "", err
	}
	return oc.AuthCodeURL(state), nil
}

// LoginAuth 执行三方登录回调：校验 state、用 code 换令牌、再换用户资料。
//
// 三步同序对照 Java AuthDefaultRequest.login：checkCode → checkState → 换令牌 → 取用户。
func LoginAuth(ctx context.Context, source, code, state string) (*AuthUser, error) {
	p, lc, err := lookup(source)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, errors.New("social: 三方登录 code 为空")
	}
	if err := checkState(ctx, state); err != nil {
		return nil, err
	}

	oc, err := p.OAuth2Config(lc)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: httpTimeout}
	// 把 client 交给 x/oauth2 走令牌交换，超时设置才对 Exchange 生效。
	tok, err := oc.Exchange(context.WithValue(ctx, oauth2.HTTPClient, client), code)
	if err != nil {
		return nil, fmt.Errorf("social: %s 换取令牌失败: %w", source, err)
	}

	user, err := p.FetchUser(ctx, lc, client, tok)
	if err != nil {
		return nil, err
	}
	if user.UUID == "" {
		// 拿不到平台内唯一标识就没法拼 auth_id，绑定关系无从建立。
		return nil, fmt.Errorf("social: %s 未返回用户唯一标识", source)
	}
	return user, nil
}

// tokenFrom 把 oauth2.Token 摊成 AuthToken。
//
// scope / id_token 不是 oauth2.Token 的固定字段，只能从 Extra 里取
// (x/oauth2 把标准字段之外的键都塞进 raw)。
func tokenFrom(tok *oauth2.Token) AuthToken {
	at := AuthToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
	}
	if s, ok := tok.Extra("scope").(string); ok {
		at.Scope = s
	}
	if s, ok := tok.Extra("id_token").(string); ok {
		at.IDToken = s
	}
	// Expiry 零值表示平台未给有效期(如 github)，此时 ExpireIn 留 0。
	if !tok.Expiry.IsZero() {
		if secs := int(time.Until(tok.Expiry).Seconds()); secs > 0 {
			at.ExpireIn = secs
		}
	}
	return at
}

// getJSON 带令牌请求一个返回 JSON 的接口并解码进 dest。
// header 用于各平台不同的授权头形态(github 是 token xxx，OIDC 系是 Bearer xxx)。
func getJSON(ctx context.Context, client *http.Client, url string,
	header http.Header, dest any) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("social: 构造请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("social: 请求 %s 失败: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("social: 读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// 带上响应体便于排查(令牌过期、scope 不足等平台都写在 body 里)。
		return fmt.Errorf("social: %s 返回 %d: %s", url, resp.StatusCode, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("social: 解析响应失败: %w, 响应: %s", err, truncate(string(body), 200))
	}
	return nil
}

// bearerHeader 构造 Authorization: Bearer <token> 头，OIDC 系平台通用。
func bearerHeader(accessToken string) http.Header {
	return http.Header{"Authorization": []string{"Bearer " + accessToken}}
}

// firstNonEmpty 返回首个非空串，对应 JustAuth 的 firstNotEmpty。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// truncate 截断过长的字符串，避免把整页 HTML 塞进日志。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// sortedKeys 取排序后的键，仅用于日志输出稳定。
func sortedKeys(m map[string]Provider) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
