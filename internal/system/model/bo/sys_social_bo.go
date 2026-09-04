package bo

// SysSocialBo 社会化关系业务对象（入参）。
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
