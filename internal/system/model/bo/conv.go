package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// goverter 生成 BO→entity 转换器，替代手写逐字段拷贝。
// 生成：go generate ./internal/system/model/...
//
//   - nil→nil：Pointer 构建器默认守卫，无需设置。
//   - ignoreMissing yes：无源对应字段（BaseEntity 审计子字段、DelFlag 等）留零。
//   - skipCopySameType yes：同类型直接拷贝，免 time.Time 未导出字段报错。
//   - map . BaseEntity（3 特例）：整个 BO 作为源填嵌入式 BaseEntity，按名写
//     CreateBy/UpdateBy（SysUser）、CreateBy（SysOss）、CreateDept（SysDictData）。
//
// 命名 ConvertTo<Target>；列表加 List；profile 用 FromProfile 消歧。
//
//go:generate go tool goverter gen .

//goverter:converter
//goverter:output:file conv_gen.go
//goverter:ignoreMissing yes
//goverter:skipCopySameType yes
type Converter interface {
	ConvertToSysClient(b *SysClientBo) *systemmodel.SysClient
	ConvertToSysClientList(in []*SysClientBo) []*systemmodel.SysClient

	ConvertToSysConfig(b *SysConfigBo) *systemmodel.SysConfig
	ConvertToSysConfigList(in []*SysConfigBo) []*systemmodel.SysConfig

	ConvertToSysDept(b *SysDeptBo) *systemmodel.SysDept
	ConvertToSysDeptList(in []*SysDeptBo) []*systemmodel.SysDept

	//goverter:map . BaseEntity
	ConvertToSysDictData(b *SysDictDataBo) *systemmodel.SysDictData
	ConvertToSysDictDataList(in []*SysDictDataBo) []*systemmodel.SysDictData

	ConvertToSysDictType(b *SysDictTypeBo) *systemmodel.SysDictType
	ConvertToSysDictTypeList(in []*SysDictTypeBo) []*systemmodel.SysDictType

	ConvertToSysLoginInfo(b *SysLoginInfoBo) *systemmodel.SysLoginInfo
	ConvertToSysLoginInfoList(in []*SysLoginInfoBo) []*systemmodel.SysLoginInfo

	ConvertToSysMenu(b *SysMenuBo) *systemmodel.SysMenu
	ConvertToSysMenuList(in []*SysMenuBo) []*systemmodel.SysMenu

	ConvertToSysNotice(b *SysNoticeBo) *systemmodel.SysNotice
	ConvertToSysNoticeList(in []*SysNoticeBo) []*systemmodel.SysNotice

	ConvertToSysOperLog(b *SysOperLogBo) *systemmodel.SysOperLog
	ConvertToSysOperLogList(in []*SysOperLogBo) []*systemmodel.SysOperLog

	//goverter:map . BaseEntity
	ConvertToSysOss(b *SysOssBo) *systemmodel.SysOss
	ConvertToSysOssList(in []*SysOssBo) []*systemmodel.SysOss

	ConvertToSysOssConfig(b *SysOssConfigBo) *systemmodel.SysOssConfig
	ConvertToSysOssConfigList(in []*SysOssConfigBo) []*systemmodel.SysOssConfig

	ConvertToSysPost(b *SysPostBo) *systemmodel.SysPost
	ConvertToSysPostList(in []*SysPostBo) []*systemmodel.SysPost

	ConvertToSysRole(b *SysRoleBo) *systemmodel.SysRole
	ConvertToSysRoleList(in []*SysRoleBo) []*systemmodel.SysRole

	ConvertToSysSocial(b *SysSocialBo) *systemmodel.SysSocial
	ConvertToSysSocialList(in []*SysSocialBo) []*systemmodel.SysSocial

	//goverter:map . BaseEntity
	ConvertToSysUser(b *SysUserBo) *systemmodel.SysUser
	ConvertToSysUserList(in []*SysUserBo) []*systemmodel.SysUser

	// SysUserProfileBo 仅含 5 列，其余由 ignoreMissing 留零。
	ConvertToSysUserFromProfile(b *SysUserProfileBo) *systemmodel.SysUser
	ConvertToSysUserFromProfileList(in []*SysUserProfileBo) []*systemmodel.SysUser
}
