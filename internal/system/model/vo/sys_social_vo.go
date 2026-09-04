package vo

import (
	"time"
)

// SysSocialVo 社会化关系视图对象。
type SysSocialVo struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"userId"`
	AuthID           string     `json:"authId"`
	Source           string     `json:"source"`
	AccessToken      string     `json:"accessToken"`
	ExpireIn         int        `json:"expireIn"`
	RefreshToken     string     `json:"refreshToken"`
	OpenID           string     `json:"openId"`
	UserName         string     `json:"userName"`
	NickName         string     `json:"nickName"`
	Email            string     `json:"email"`
	Avatar           string     `json:"avatar"`
	AccessCode       string     `json:"accessCode"`
	UnionID          string     `json:"unionId"`
	Scope            string     `json:"scope"`
	TokenType        string     `json:"tokenType"`
	IDToken          string     `json:"idToken"`
	MacAlgorithm     string     `json:"macAlgorithm"`
	MacKey           string     `json:"macKey"`
	Code             string     `json:"code"`
	OauthToken       string     `json:"oauthToken"`
	OauthTokenSecret string     `json:"oauthTokenSecret"`
	CreateTime       *time.Time `json:"createTime"`
}
