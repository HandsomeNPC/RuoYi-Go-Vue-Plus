package constant

import "time"

// 缓存组名，移植原项目 CacheNames。
//
// 原项目组名形如 "sys_client#30d"，'#' 后是 Redisson 的缓存策略参数
// （ttl#maxIdleTime#maxSize#local），只用于构造 CacheConfig，真正传给
// getMap() 的是 '#' 之前的那一段。所以这里只保留组名本身，TTL 拆到下面的
// CacheTTLXxx —— 若把整串当组名，会写出名为 `sys_client#30d` 的 Hash，
// 与原项目数据对不上。
const (
	CacheDemoCache       = "demo:cache"         // 演示案例
	CacheSysConfig       = "sys_config"         // 参数配置，field=configKey
	CacheSysDict         = "sys_dict"           // 字典数据，field=dictType
	CacheSysDictType     = "sys_dict_type"      // 字典类型，field=dictType
	CacheSysClient       = "sys_client"         // 客户端，field=clientId
	CacheSysUserName     = "sys_user_name"      // 用户账号，field=userId
	CacheSysNickname     = "sys_nickname"       // 用户昵称，field=userId
	CacheSysDept         = "sys_dept"           // 部门，field=deptId
	CacheSysOss          = "sys_oss"            // OSS 内容，field=ossId
	CacheSysRoleCustom   = "sys_role_custom"    // 角色自定义数据权限，field=roleId
	CacheSysDeptAndChild = "sys_dept_and_child" // 部门及以下部门 ID，field=deptId
	CacheSysOssConfig    = "sys_oss_config"     // OSS 配置，field=configKey
)

// Redis key 前缀。与上面的「缓存组」不同：这两个不是 Hash 组名，
// 而是独立 key 的前缀，使用时直接拼接标识（原项目同样是字符串拼接）。
const (
	OnlineTokenKeyPrefix = "online_tokens:" // 在线用户，后接 token
	PwdErrCntKeyPrefix   = "pwd_err_cnt:"   // 登录密码错误次数，后接 username
)

// 缓存组过期时间，取自原项目组名 '#' 后的第一段参数。
//
// 未列出的组（sys_config / sys_dict / sys_dict_type / sys_oss_config）原声明
// 不带 '#' 参数，即 TTL=0 —— 按 Redisson 语义是**永不过期**，靠业务在增删改时
// 主动 evict 保证一致性。Go 侧同理：这几组写入不要设过期时间，改数据时记得清缓存。
const (
	// CacheTTLDemoCache 对应原 "demo:cache#60s#10m#20"，ttl 段是 60s。
	// 后两段（maxIdleTime=10m 空闲淘汰、maxSize=20 组容量上限）依赖 Redisson
	// 的本地缓存能力，Go 侧没有等价物，暂不实现——demo 组本身无业务影响。
	CacheTTLDemoCache = 60 * time.Second

	CacheTTLSysClient       = 30 * 24 * time.Hour
	CacheTTLSysUserName     = 30 * 24 * time.Hour
	CacheTTLSysNickname     = 30 * 24 * time.Hour
	CacheTTLSysDept         = 30 * 24 * time.Hour
	CacheTTLSysOss          = 30 * 24 * time.Hour
	CacheTTLSysRoleCustom   = 30 * 24 * time.Hour
	CacheTTLSysDeptAndChild = 30 * 24 * time.Hour
)
