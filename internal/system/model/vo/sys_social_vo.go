package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysSocialVo 社会化关系视图对象，对应 Java SysSocialVo。
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

// FromSysSocial 把实体转成 VO。
func FromSysSocial(s *systemmodel.SysSocial) *SysSocialVo {
	if s == nil {
		return nil
	}
	return &SysSocialVo{
		ID:               s.ID,
		UserID:           s.UserID,
		AuthID:           s.AuthID,
		Source:           s.Source,
		AccessToken:      s.AccessToken,
		ExpireIn:         s.ExpireIn,
		RefreshToken:     s.RefreshToken,
		OpenID:           s.OpenID,
		UserName:         s.UserName,
		NickName:         s.NickName,
		Email:            s.Email,
		Avatar:           s.Avatar,
		AccessCode:       s.AccessCode,
		UnionID:          s.UnionID,
		Scope:            s.Scope,
		TokenType:        s.TokenType,
		IDToken:          s.IDToken,
		MacAlgorithm:     s.MacAlgorithm,
		MacKey:           s.MacKey,
		Code:             s.Code,
		OauthToken:       s.OauthToken,
		OauthTokenSecret: s.OauthTokenSecret,
		CreateTime:       s.CreateTime,
	}
}
