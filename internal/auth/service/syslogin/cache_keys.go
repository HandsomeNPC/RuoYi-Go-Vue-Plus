// Package syslogin 登录态与登录日志。
package syslogin

// Redis 键前缀。两者都由 auth 在登录流程中写入，其他模块只读：
// 在线摘要由 monitor 的在线用户列表读，密码错误次数由 system(解锁)
// 与 monitor(解锁登录记录)读，故前缀定义跟着写入方走。
const (
	// OnlineTokenKeyPrefix 在线会话摘要，后接 token。
	//
	// 与 satoken 包内的同值常量是两份：那边是会话存储自己的键空间，
	// 这边是本项目自管的在线摘要键，互不依赖。
	OnlineTokenKeyPrefix = "online_tokens:"

	// PwdErrCntKeyPrefix 密码错误次数，后接用户名。
	PwdErrCntKeyPrefix = "pwd_err_cnt:"

	// SocialAuthCodeKey 三方认证 state，后接 state 值。
	SocialAuthCodeKey = "global:social_auth_codes:"
)
