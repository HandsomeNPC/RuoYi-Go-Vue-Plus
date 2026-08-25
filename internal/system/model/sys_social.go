package model

// SysSocial 社会化关系表（sys_social），对应 Java org.dromara.system.domain.SysSocial。
//
// 字段与 Java 实体对齐；DB 里另有 del_flag 列由框架逻辑删除使用，Java 实体未显式声明，
// 此处亦不映射，避免与逻辑删除策略耦合（后续引入软删除时再补）。
type SysSocial struct {
	ID               int64  `gorm:"column:id;primaryKey" json:"id"`
	UserID           int64  `gorm:"column:user_id" json:"userId"`
	AuthID           string `gorm:"column:auth_id" json:"authId"`
	Source           string `gorm:"column:source" json:"source"`
	AccessToken      string `gorm:"column:access_token" json:"accessToken"`
	ExpireIn         int    `gorm:"column:expire_in" json:"expireIn"`
	RefreshToken     string `gorm:"column:refresh_token" json:"refreshToken"`
	OpenID           string `gorm:"column:open_id" json:"openId"`
	UserName         string `gorm:"column:user_name" json:"userName"`
	NickName         string `gorm:"column:nick_name" json:"nickName"`
	Email            string `gorm:"column:email" json:"email"`
	Avatar           string `gorm:"column:avatar" json:"avatar"`
	AccessCode       string `gorm:"column:access_code" json:"accessCode"`
	UnionID          string `gorm:"column:union_id" json:"unionId"`
	Scope            string `gorm:"column:scope" json:"scope"`
	TokenType        string `gorm:"column:token_type" json:"tokenType"`
	IDToken          string `gorm:"column:id_token" json:"idToken"`
	MacAlgorithm     string `gorm:"column:mac_algorithm" json:"macAlgorithm"`
	MacKey           string `gorm:"column:mac_key" json:"macKey"`
	Code             string `gorm:"column:code" json:"code"`
	OauthToken       string `gorm:"column:oauth_token" json:"oauthToken"`
	OauthTokenSecret string `gorm:"column:oauth_token_secret" json:"oauthTokenSecret"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysSocial) TableName() string {
	return "sys_social"
}
