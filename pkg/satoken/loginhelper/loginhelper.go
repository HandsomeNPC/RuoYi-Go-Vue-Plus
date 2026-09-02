// Package loginhelper 登录鉴权助手
package loginhelper

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/stputil"

	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/enum"
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

	sess, err := stputil.GetTokenSessionOrCreate(token)
	if err != nil {
		return "", err
	}
	// 存 JSON 串而非结构体指针：Session.Data 是 map[string]any 整体 Marshal 进 Redis，
	// 且库里没有类型注册表，直接存指针跨请求取回来是 map[string]any，断言必失败。
	payload, err := json.Marshal(loginUser)
	if err != nil {
		return "", err
	}
	if err := sess.Set(LoginUserKey, string(payload)); err != nil {
		return "", err
	}

	// sa-token 的权限/角色校验只认 account session 的 permissions/roles 键，
	// 与上面 token session 的 loginUser 是两套存储，不双写则任何权限码恒判否。
	// 这里存原生 []string 而非 JSON 串：取回侧 Manager.toStringSlice 已处理 []any 分支。
	if err := stputil.SetPermissions(loginID, toSaTokenPerms(loginUser.MenuPermission)); err != nil {
		return "", err
	}
	if err := stputil.SetRoles(loginID, loginUser.RolePermission); err != nil {
		return "", err
	}
	return token, nil
}

// toSaTokenPerms 把超管的 *:*:* 换成 sa-token 认得的 *，其余原样。
//
// sa-token-go 的 matchPermission 对 *:*:* 会先命中「以 :* 结尾」的前缀分支，
// 去掉尾部 * 后拿 "*:*:" 去 HasPrefix 实际权限码——必然为假且提前返回，
// 走不到后面按段比对的分支，于是超管反而处处被拒。而它对单个 * 是直接放行的。
// 只在写入这一侧转换：LoginUser.MenuPermission 仍是 *:*:*，前端 hasPermi 按字面量比对。
func toSaTokenPerms(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if p == constant.AllPermission {
			p = "*"
		}
		out = append(out, p)
	}
	return out
}

// loginUserCacheKey gin.Context 中本请求登录用户快照的键。
const loginUserCacheKey = "ruoyi:loginUser"

// GetLoginUser 取当前请求的登录用户，未登录返回 nil。
//
// 结果按请求缓存进 gin.Context：每次都要解会话就是一次 Redis 往返，而一个请求里
// 审计中间件、操作日志、handler 各自都要取一遍。Java 侧 StpUtil 的会话有 sa-token
// 的请求级 holder 兜着，sa-token-go 没有，只能在这里自己存一层。
// 未登录也缓存（存 nil），否则匿名请求每个调用点都白跑一次 Redis。
//
// 返回的是本请求内共享的同一个指针，调用方只读、不要改字段。
func GetLoginUser(c *gin.Context) *authmodel.LoginUser {
	if v, ok := c.Get(loginUserCacheKey); ok {
		// 未登录时存的是 nil，断言失败正好落回 nil。
		lu, _ := v.(*authmodel.LoginUser)
		return lu
	}
	lu := GetLoginUserByToken(sagin.GetTokenFromCtx(c))
	c.Set(loginUserCacheKey, lu)
	return lu
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
	v := sess.GetString(LoginUserKey)
	if v == "" {
		return nil
	}
	var lu authmodel.LoginUser
	if err := json.Unmarshal([]byte(v), &lu); err != nil {
		return nil
	}
	return &lu
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

// LogoutUser 注销指定用户的会话（对应 Java StpUtil.logoutByLoginId(loginID)，
// 即 OnlineUserCleanEvent 的最终动作：角色/授权变动后让受影响在线用户下次请求即失效）。
//
// 只注销 default 设备的会话：sa-token-go 的 account→token 映射按 (loginID, device) 分桶，
// 没有现成的"枚举该用户全部设备"接口，多端登录的其余会话不会被一并清掉。这是与
// Java searchTokenValue 全量扫描的有意差距——单端覆盖已能让权限变更在下次登录生效，
// 多端场景留待 sa-token-go 提供枚举能力后再补。
//
// 失败只记日志：注销失败不该阻断写库流程，与 Java 的 try/catch ignored 一致。
func LogoutUser(userID int64) {
	if userID <= 0 {
		return
	}
	loginID := enum.UserTypeSys.Code + ":" + strconv.FormatInt(userID, 10)
	if err := sagin.Logout(loginID); err != nil {
		log.Printf("[satoken] 注销用户 %d 会话失败: %v", userID, err)
	}
}

// LogoutUsers 批量注销用户会话，逐个调用 LogoutUser。
// 不做去重：sys_user_role 不会给同一用户重复行，调用方传入的集合语义上也无重复预期。
func LogoutUsers(userIDs []int64) {
	for _, id := range userIDs {
		LogoutUser(id)
	}
}
