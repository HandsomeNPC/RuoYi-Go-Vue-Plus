// Package auth 登录态原语：LoginUser 模型、JWT 签发/校验、Redis 会话、密码哈希。
//
// 对应原项目 sa-token 那一套（ruoyi-common-satoken）：
//
//	LoginHelper.login / getLoginUser  → 本包的 Sign + Session
//	StpLogicJwtForSimple              → JWT 携带身份 + Redis 存会话载荷
//	BCrypt.checkpw / hashpw           → password.go
//
// **有意不 import gin**：internal/*/service 要签发与销毁会话，那条路径与 HTTP
// 无关。分包理由与 pkg/encrypt 同源 —— 那边把加解密原语与「谁该加密」的策略
// 分开，这里把「登录态是什么」与「怎么从请求里校验它」分开，后者在
// pkg/middleware/auth.go。
package auth

import (
	"strconv"

	"ruoyi-go-vue-plus/pkg/enum"
)

// LoginUser 登录用户信息，对应原项目 org.dromara.system.api.model.LoginUser。
//
// 它是**会话载荷**：整个对象序列化后存进 Redis（键 auth:token:<jwt>），
// 与 JWT 里的 claims 是两份东西。分工见 claims.go 的说明。
//
// 字段对齐 Java 侧 22 个。本阶段（阶段 1）只填得出其中一部分：
// MenuPermission / RolePermission / Roles / Posts 需要 sys_menu / sys_role /
// sys_post，DeptName / DeptCategory 需要 sys_dept，都是阶段 2 的表。
// 现在**保留字段但留空**而不是等阶段 2 再加：这些字段会进 Redis 里的 JSON，
// 后加字段意味着阶段 2 上线时所有存量会话反序列化后少字段，
// 而那种「登录着的用户突然没有权限」的故障只在灰度期出现、极难复现。
type LoginUser struct {
	UserID   int64  `json:"userId"`
	DeptID   int64  `json:"deptId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	// UserType 用户类型，取值见 enum.UserType（sys_user / app_user）。
	// 与 UserID 一起拼成 LoginID，是会话的主体标识。
	UserType string `json:"userType"`

	// DeptName / DeptCategory 需要 sys_dept，阶段 2 填。
	DeptName     string `json:"deptName"`
	DeptCategory string `json:"deptCategory"`

	// Token 本次会话的 JWT，对应 Java LoginUser.token。
	Token string `json:"token"`
	// LoginTime / ExpireTime 毫秒时间戳，对齐 Java 的 Long 类型。
	LoginTime  int64 `json:"loginTime"`
	ExpireTime int64 `json:"expireTime"`

	// 登录来源信息，对应 Java LoginHelper.fillRequestContext 填的那几项。
	// LoginLocation（IP 归属地）需要 ip2region 库，本项目暂不实现，恒为空。
	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`

	// MenuPermission / RolePermission 权限码集合，阶段 2 填。
	// 用 []string 而非 Java 的 Set：Go 无内建 set，而这两个集合的用途是
	// 「判断是否包含某个权限码」，阶段 2 接权限校验时再决定要不要转成 map。
	MenuPermission []string `json:"menuPermission"`
	RolePermission []string `json:"rolePermission"`

	// Roles / Posts / DataScopeRoleMap / RoleID 数据权限相关，阶段 2 与 4.1 填。
	Roles            []RoleInfo         `json:"roles"`
	Posts            []PostInfo         `json:"posts"`
	DataScopeRoleMap map[string][]int64 `json:"dataScopeRoleMap"`
	RoleID           int64              `json:"roleId"`

	// ClientKey / DeviceType 来自 sys_client，登录时由策略填入。
	// DeviceType 是**自由字符串**不是枚举 —— 种子数据里 app 客户端是
	// "android"，不在 enum.DeviceType 的取值域内（详见那个类型的注释）。
	ClientKey  string `json:"clientKey"`
	DeviceType string `json:"deviceType"`
}

// RoleInfo 角色摘要，对应 Java 的 RoleDTO。阶段 2 填充。
type RoleInfo struct {
	RoleID    int64  `json:"roleId"`
	RoleName  string `json:"roleName"`
	RoleKey   string `json:"roleKey"`
	DataScope string `json:"dataScope"`
}

// PostInfo 岗位摘要，对应 Java 的 PostDTO。阶段 2 填充。
type PostInfo struct {
	PostID   int64  `json:"postId"`
	PostName string `json:"postName"`
	PostCode string `json:"postCode"`
}

// LoginID 返回会话主体标识，形如 "sys_user:1761100000000000001"。
//
// 对应 Java LoginUser.getLoginId()，是 JWT 的 sub。用「类型:ID」而非裸 ID
// 是因为同一套用户表可承载多种用户体系（后台用户 / App 用户），
// 权限体系不同，只看 ID 会串号。
//
// 与 Java 的差异：那边 userType 或 userId 为空时抛 IllegalArgumentException，
// 这里返回 ok=false 交调用方处置 —— 签发路径上这是编程错误（构造 LoginUser
// 时漏填），返回错误比 panic 更符合 Go 惯例，也不会让一个畸形的登录请求
// 打挂进程。解析方向见 ParseLoginID。
func (u *LoginUser) LoginID() (string, bool) {
	if u == nil || u.UserType == "" || u.UserID == 0 {
		return "", false
	}
	return u.UserType + ":" + strconv.FormatInt(u.UserID, 10), true
}

// ParseLoginID 从会话主体标识中拆出用户类型与用户 ID。
//
// 按**第一个**冒号切分，不是最后一个：用户类型里不含冒号，而将来若 ID 段
// 出现冒号（比如拼上设备标识），从后切会把类型段割坏。对齐 Java
// SaPermissionImpl.resolveUserId 取 indexOf(":") 之后的做法。
//
// 用户类型只做非空校验、不查 enum.ParseUserType 白名单：那个枚举的取值域
// 随业务扩展（阶段 4 的 app 端登录），在这里卡白名单会让新增用户类型
// 需要同时改两处，而漏改的表现是「登录成功但每个请求都 401」。
func ParseLoginID(loginID string) (userType string, userID int64, ok bool) {
	for i := 0; i < len(loginID); i++ {
		if loginID[i] != ':' {
			continue
		}
		userType = loginID[:i]
		if userType == "" {
			return "", 0, false
		}
		id, err := strconv.ParseInt(loginID[i+1:], 10, 64)
		if err != nil || id == 0 {
			return "", 0, false
		}
		return userType, id, true
	}
	return "", 0, false
}

// IsSuperAdmin 判断是否超级管理员，对应 LoginHelper.isSuperAdmin。
//
// 用常量 ID 比对而非查角色表，对齐原项目 —— 超管是**内置**身份，
// 不依赖 sys_role 的数据，删掉超管角色也不该让系统失去管理入口。
func (u *LoginUser) IsSuperAdmin() bool {
	return u != nil && u.UserID == superAdminUserID
}

// superAdminUserID 超级管理员用户 ID。
//
// 值取自 pkg/constant.SuperAdminUserID。这里重新声明而非直接 import constant，
// 是为了让本包保持无内部依赖（便于将来独立复用）；两者一致由
// TestSuperAdminIDMatchesConstant 锁住。
const superAdminUserID int64 = 1761100000000000001

// UserTypeSys 后台系统用户的类型标识，签发会话时的默认值。
//
// 别名指向 enum，避免调用方为了一个字符串常量去 import enum。
var UserTypeSys = enum.UserTypeSys.Code
