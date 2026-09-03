package social

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"

	"ruoyi-go-vue-plus/pkg/config"
)

// serverEndpoints 私有化部署平台的端点模板。
// %s 由配置里的 serverUrl 填充，取值逐字对齐 JustAuth AuthDefaultSource。
var serverEndpoints = map[string]struct {
	authorize string
	token     string
	userInfo  string
	// authStyle client 凭据的传递方式：topiam 走 HTTP Basic，maxkey 放参数体。
	// 这两种形态的差异由 x/oauth2 处理，不必手写签名。
	authStyle oauth2.AuthStyle
	// defaultScopes 平台默认权限范围，配置未指定 scopes 时启用。
	defaultScopes []string
}{
	SourceMaxkey: {
		authorize: "%s/sign/authz/oauth/v20/authorize",
		token:     "%s/sign/authz/oauth/v20/token",
		userInfo:  "%s/sign/api/oauth/v20/me",
		authStyle: oauth2.AuthStyleInParams,
	},
	SourceTopiam: {
		authorize:     "%s/oauth2/auth",
		token:         "%s/oauth2/token",
		userInfo:      "%s/oauth2/userinfo",
		authStyle:     oauth2.AuthStyleInHeader,
		defaultScopes: []string{"openid", "profile", "email"},
	},
}

// serverProvider 支持私有化部署的平台(maxkey / topiam)。
//
// 这些平台的端点不是固定 URL 而是 serverUrl 套模板，故一份实现覆盖多家，
// 差异全在 serverEndpoints 表与 FetchUser 的字段映射里。
type serverProvider struct {
	source string
}

func (p serverProvider) OAuth2Config(cfg config.SocialLoginConfig) (*oauth2.Config, error) {
	ep, ok := serverEndpoints[p.source]
	if !ok {
		return nil, fmt.Errorf("social: 平台 %s 无端点定义", p.source)
	}
	base, err := normalizeServerURL(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("social: 平台 %s 的 serverUrl 无效: %w", p.source, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = ep.defaultScopes
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   fmt.Sprintf(ep.authorize, base),
			TokenURL:  fmt.Sprintf(ep.token, base),
			AuthStyle: ep.authStyle,
		},
	}, nil
}

func (p serverProvider) FetchUser(ctx context.Context, cfg config.SocialLoginConfig,
	client *http.Client, tok *oauth2.Token) (*AuthUser, error) {

	base, err := normalizeServerURL(cfg.ServerURL)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf(serverEndpoints[p.source].userInfo, base)

	// 两家的字段名不同，故收全所有候选键，再按平台各自的优先级取。
	var u struct {
		Sub               string `json:"sub"`
		UserID            string `json:"userId"`
		Userid            string `json:"userid"`
		Username          string `json:"username"`
		PreferredUsername string `json:"preferred_username"`
		DisplayName       string `json:"displayName"`
		Nickname          string `json:"nickname"`
		Name              string `json:"name"`
		AvatarURL         string `json:"avatar_url"`
		Picture           string `json:"picture"`
		Email             string `json:"email"`
	}
	if err := getJSON(ctx, client, endpoint, bearerHeader(tok.AccessToken), &u); err != nil {
		return nil, err
	}

	au := &AuthUser{Source: p.source, Email: u.Email, Token: tokenFrom(tok)}
	// 取值优先级逐字对齐 JustAuth 的 AuthMaxKeyRequest / AuthTopIamRequest。
	switch p.source {
	case SourceMaxkey:
		au.UUID = firstNonEmpty(u.UserID, u.Userid, u.Sub)
		au.Username = u.Username
		au.Nickname = u.DisplayName
		au.Avatar = u.AvatarURL
	case SourceTopiam:
		au.UUID = u.Sub
		au.Username = u.PreferredUsername
		au.Nickname = firstNonEmpty(u.Nickname, u.Name, u.PreferredUsername)
		au.Avatar = u.Picture
	}
	return au, nil
}

// normalizeServerURL 校验 serverUrl 并剥掉尾部斜杠。
//
// 必须剥：端点模板形如 "%s/oauth2/auth"，serverUrl 带尾斜杠会拼出 "//oauth2/auth"。
// 对齐 JustAuth AbstractAuthServerRequest.normalizeServerUrl 与
// AuthChecker.isValidServerUrl(要求 http/https、有 host、无 query/fragment)。
func normalizeServerURL(serverURL string) (string, error) {
	if serverURL == "" {
		return "", fmt.Errorf("serverUrl 未配置")
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("serverUrl 须以 http:// 或 https:// 开头: %s", serverURL)
	}
	if u.Host == "" {
		return "", fmt.Errorf("serverUrl 缺少主机名: %s", serverURL)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("serverUrl 不应带查询串或片段: %s", serverURL)
	}
	return strings.TrimRight(serverURL, "/"), nil
}
