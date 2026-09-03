package social

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"

	"ruoyi-go-vue-plus/pkg/config"
)

// githubProvider GitHub。
//
// 端点与 x/oauth2/endpoints.GitHub 一致，但不 import 那个包：
// 它只是三行 URL 常量的别名，为一个 var 多引一个包不值当，
// 且写在此处与其余平台形状统一(gitee/maxkey/topiam 都得自己写)。
type githubProvider struct{}

func (githubProvider) OAuth2Config(cfg config.SocialLoginConfig) (*oauth2.Config, error) {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		// 不设默认 scope：JustAuth 的 AuthGithubScope 全部 isDefault=false，
		// getScopes 因而返回空串，授权地址不带 scope 参数。
		Scopes: cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}, nil
}

func (githubProvider) FetchUser(ctx context.Context, _ config.SocialLoginConfig,
	client *http.Client, tok *oauth2.Token) (*AuthUser, error) {

	var u struct {
		ID     json.Number `json:"id"`
		Login  string      `json:"login"`
		Name   string      `json:"name"`
		Avatar string      `json:"avatar_url"`
		Email  string      `json:"email"`
	}
	// 授权头是 "token xxx" 而非 "Bearer xxx"，对齐 JustAuth TokenUtils.token。
	header := http.Header{"Authorization": []string{"token " + tok.AccessToken}}
	if err := getJSON(ctx, client, "https://api.github.com/user", header, &u); err != nil {
		return nil, err
	}

	return &AuthUser{
		UUID:     u.ID.String(),
		Source:   SourceGithub,
		Username: u.Login,
		Nickname: u.Name,
		Avatar:   u.Avatar,
		Email:    u.Email,
		Token:    tokenFrom(tok),
	}, nil
}
