// Package loginhelper 登录鉴权助手
package loginhelper

import (
	"strconv"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/stputil"

	"ruoyi-go-vue-plus/pkg/constant"
	authmodel "ruoyi-go-vue-plus/pkg/model"
)

// LoginUserKey token session 中 *LoginUser 的键。
const LoginUserKey = "loginUser"

// Login 签发令牌、把 *LoginUser 存入 token session，并回填 loginUser.Token。
// device 对应 SaLoginParameter.deviceType，"" 走默认。
func Login(loginUser *authmodel.LoginUser, device string) (string, error) {
	loginID, err := loginUser.LoginID()
	if err != nil {
		return "", err
	}
	token, err := sagin.Login(loginID, device)
	if err != nil {
		return "", err
	}
	loginUser.Token = token

	// 必须用 OrCreate：sagin.Login 只签发 token、写登录态，不建 token-session。
	// 只读的 GetTokenSession 在 session 不存在时返回 (nil, nil)——nil 却不报错，
	// 后面 sess.Set 就会空指针。gin 集成层未导出 OrCreate，故直接走 stputil。
	sess, err := stputil.GetTokenSessionOrCreate(token)
	if err != nil {
		return "", err
	}
	if err := sess.Set(LoginUserKey, loginUser); err != nil {
		return "", err
	}
	return token, nil
}

// GetLoginUser 取当前请求的登录用户，未登录返回 nil。
func GetLoginUser(c *gin.Context) *authmodel.LoginUser {
	return GetLoginUserByToken(sagin.GetTokenFromCtx(c))
}

// GetLoginUserByToken 按指定 token 取登录用户。
func GetLoginUserByToken(token string) *authmodel.LoginUser {
	if token == "" {
		return nil
	}
	sess, err := sagin.GetTokenSession(token)
	if err != nil || sess == nil {
		return nil
	}
	v, ok := sess.Get(LoginUserKey)
	if !ok {
		return nil
	}
	lu, ok := v.(*authmodel.LoginUser)
	if !ok {
		return nil
	}
	return lu
}

// GetLoginID 取当前 token 的 loginID（形如 "sys_user:123"），未登录返回 ""。
func GetLoginID(c *gin.Context) string {
	token := sagin.GetTokenFromCtx(c)
	if token == "" {
		return ""
	}
	id, err := sagin.GetLoginID(token)
	if err != nil {
		return ""
	}
	return id
}

// GetUserID 取当前登录用户 ID，未登录返回 0。
func GetUserID(c *gin.Context) int64 {
	if lu := GetLoginUser(c); lu != nil {
		return lu.UserID
	}
	return 0
}

// GetUserIDStr 取当前登录用户 ID 字符串，未登录返回 ""。
func GetUserIDStr(c *gin.Context) string {
	if id := GetUserID(c); id != 0 {
		return strconv.FormatInt(id, 10)
	}
	return ""
}

// GetUsername 取当前登录用户名，未登录返回 ""。
func GetUsername(c *gin.Context) string {
	if lu := GetLoginUser(c); lu != nil {
		return lu.Username
	}
	return ""
}

// GetDeptID 取当前登录用户部门 ID，未登录返回 0。
func GetDeptID(c *gin.Context) int64 {
	if lu := GetLoginUser(c); lu != nil {
		return lu.DeptID
	}
	return 0
}

// GetDeptName 取当前登录用户部门名，未登录返回 ""。
func GetDeptName(c *gin.Context) string {
	if lu := GetLoginUser(c); lu != nil {
		return lu.DeptName
	}
	return ""
}

// GetDeptCategory 取当前登录用户部门类别编码，未登录返回 ""。
func GetDeptCategory(c *gin.Context) string {
	if lu := GetLoginUser(c); lu != nil {
		return lu.DeptCategory
	}
	return ""
}

// GetUserType 取当前登录用户类型，未登录返回 ""。
func GetUserType(c *gin.Context) string {
	if lu := GetLoginUser(c); lu != nil {
		return lu.UserType
	}
	return ""
}

// IsSuperAdmin 判断指定用户 ID 是否超级管理员。
func IsSuperAdmin(userID int64) bool {
	return userID == constant.SuperAdminUserID
}

// IsCurrentSuperAdmin 判断当前登录用户是否超级管理员。
func IsCurrentSuperAdmin(c *gin.Context) bool {
	return IsSuperAdmin(GetUserID(c))
}

// IsLogin 判断当前请求是否携带有效登录态。
func IsLogin(c *gin.Context) bool {
	token := sagin.GetTokenFromCtx(c)
	return token != "" && sagin.IsLogin(token)
}
