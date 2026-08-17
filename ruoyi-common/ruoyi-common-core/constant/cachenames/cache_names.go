// Package cachenames 缓存组名称常量，统一约定缓存名和缓存策略配置格式。
//
// key 格式为 cacheNames#ttl#maxIdleTime#maxSize#local
//
// ttl 过期时间 如果设置为0则不过期 默认为0
// maxIdleTime 最大空闲时间 根据LRU算法清理空闲数据 如果设置为0则不检测 默认为0
// maxSize 组最大长度 根据LRU算法清理溢出数据 如果设置为0则无限长 默认为0
// local 默认开启本地缓存为1 关闭本地缓存为0
//
// 例子: test#60s、test#0#60s、test#0#1m#1000、test#1h#0#500、test#1h#0#500#0
//
// @author Lion Li
package cachenames

const (
	// DemoCache 演示案例
	DemoCache = "demo:cache#60s#10m#20"

	// SysConfig 系统配置
	SysConfig = "sys_config"

	// SysDict 数据字典
	SysDict = "sys_dict"

	// SysDictType 数据字典类型
	SysDictType = "sys_dict_type"

	// SysClient 客户端
	SysClient = "sys_client#30d"

	// SysUserName 用户账户
	SysUserName = "sys_user_name#30d"

	// SysNickname 用户昵称
	SysNickname = "sys_nickname#30d"

	// SysDept 部门
	SysDept = "sys_dept#30d"

	// SysOss OSS内容
	SysOss = "sys_oss#30d"

	// SysRoleCustom 角色自定义权限
	SysRoleCustom = "sys_role_custom#30d"

	// SysDeptAndChild 部门及以下权限
	SysDeptAndChild = "sys_dept_and_child#30d"

	// SysOssConfig OSS配置
	SysOssConfig = "sys_oss_config"

	// OnlineTokenKey 在线用户
	OnlineTokenKey = "online_tokens:"

	// PwdErrCntKey 登录账户密码错误次数 redis key
	PwdErrCntKey = "pwd_err_cnt:"
)
