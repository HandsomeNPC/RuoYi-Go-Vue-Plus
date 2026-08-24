package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷。
type Claims struct {
	jwt.RegisteredClaims

	UserID       int64  `json:"userId"`
	Username     string `json:"userName"`
	DeptID       int64  `json:"deptId"`
	DeptName     string `json:"deptName"`
	DeptCategory string `json:"deptCategory"`

	ClientID          string `json:"clientid"`
	ClientAccessPath  string `json:"clientAccessPath"`
	ClientIPWhitelist string `json:"clientIpWhitelist"`
}
