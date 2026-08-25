package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysSocialBo 社会化关系业务对象（入参），对应 Java SysSocialBo。
type SysSocialBo struct {
	ID               int64  `json:"id"`
	AuthID           string `json:"authId" binding:"required"`
	Source           string `json:"source" binding:"required"`
	AccessToken      string `json:"accessToken" binding:"required"`
	ExpireIn         int    `json:"expireIn"`
	RefreshToken     string `json:"refreshToken"`
	OpenID           string `json:"openId"`
	UserID           int64  `json:"userId" binding:"required"`
	AccessCode       string `json:"accessCode"`
	UnionID          string `json:"unionId"`
	Scope            string `json:"scope"`
	UserName         string `json:"userName"`
	NickName         string `json:"nickName"`
	Email            string `json:"email"`
	Avatar           string `json:"avatar"`
	TokenType        string `json:"tokenType"`
	IDToken          string `json:"idToken"`
	MacAlgorithm     string `json:"macAlgorithm"`
	MacKey           string `json:"macKey"`
	Code             string `json:"code"`
	OauthToken       string `json:"oauthToken"`
	OauthTokenSecret string `json:"oauthTokenSecret"`
}

// ToSysSocial 把 BO 转成实体。
func (b *SysSocialBo) ToSysSocial() *systemmodel.SysSocial {
	if b == nil {
		return nil
	}
	return &systemmodel.SysSocial{
		ID:               b.ID,
		UserID:           b.UserID,
		AuthID:           b.AuthID,
		Source:           b.Source,
		AccessToken:      b.AccessToken,
		ExpireIn:         b.ExpireIn,
		RefreshToken:     b.RefreshToken,
		OpenID:           b.OpenID,
		UserName:         b.UserName,
		NickName:         b.NickName,
		Email:            b.Email,
		Avatar:           b.Avatar,
		AccessCode:       b.AccessCode,
		UnionID:          b.UnionID,
		Scope:            b.Scope,
		TokenType:        b.TokenType,
		IDToken:          b.IDToken,
		MacAlgorithm:     b.MacAlgorithm,
		MacKey:           b.MacKey,
		Code:             b.Code,
		OauthToken:       b.OauthToken,
		OauthTokenSecret: b.OauthTokenSecret,
	}
}
