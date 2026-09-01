package constant

import "time"

// 缓存组名。
const (
	CacheDemoCache       = "demo:cache"
	CacheSysConfig       = "sys_config"
	CacheSysDict         = "sys_dict"
	CacheSysDictType     = "sys_dict_type"
	CacheSysClient       = "sys_client"
	CacheSysUserName     = "sys_user_name"
	CacheSysNickname     = "sys_nickname"
	CacheSysDept         = "sys_dept"
	CacheSysOss          = "sys_oss"
	CacheSysRoleCustom   = "sys_role_custom"
	CacheSysDeptAndChild = "sys_dept_and_child"
	CacheSysOssConfig    = "sys_oss_config"
)

// 独立 key 前缀，使用时直接拼接标识。
const (
	OnlineTokenKeyPrefix = "online_tokens:"
	PwdErrCntKeyPrefix   = "pwd_err_cnt:"
)

// 缓存组过期时间。
const (
	CacheTTLDemoCache = 60 * time.Second
	// CacheTTLSysConfig 参数配置不过期，对照 Java CacheNames.SYS_CONFIG 未带 #ttl 后缀。
	// 参数值极少变动且写路径都带缓存维护（新增/修改回写、删除失效），无需靠 TTL 兜底。
	CacheTTLSysConfig       = 0
	CacheTTLSysClient       = 30 * 24 * time.Hour
	CacheTTLSysUserName     = 30 * 24 * time.Hour
	CacheTTLSysNickname     = 30 * 24 * time.Hour
	CacheTTLSysDept         = 30 * 24 * time.Hour
	CacheTTLSysOss          = 30 * 24 * time.Hour
	CacheTTLSysRoleCustom   = 30 * 24 * time.Hour
	CacheTTLSysDeptAndChild = 30 * 24 * time.Hour
)
