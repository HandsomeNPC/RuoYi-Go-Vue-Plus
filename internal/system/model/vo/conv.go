package vo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// goverter 生成 entity→VO 转换器，替代手写逐字段拷贝。
// 生成：go generate ./internal/system/model/...
//
//   - nil→nil：Pointer 构建器默认守卫，无需设置。
//   - ignoreMissing yes：无源对应字段（回填字段、DelFlag 等）留零。
//   - skipCopySameType yes：*time.Time 等同类型直接拷贝，免深度转换报错。
//   - autoMap BaseEntity：提升嵌入式审计字段，使 VO 的 CreateTime/UpdateTime 按名映射。
//   - ignore Children（Dept/Menu）：源实体为扁平表无 Children 列，留零由 service 构树回填。
//
// 切片元素约定：
// VO 包内凡元素为 VO 结构体（名以 Vo 结尾）的切片，一律用指针切片 []*XxxVo，
// 与 goverter 生成的列表转换器返回类型对齐，杜绝 []*T↔[]T 转换样板代码。
//
// 命名 ConvertTo<Target>；列表加 List。
//
//go:generate go tool goverter gen .

//goverter:converter
//goverter:output:file conv_gen.go
//goverter:ignoreMissing yes
//goverter:skipCopySameType yes
type Converter interface {
	ConvertToSysClientVo(c *systemmodel.SysClient) *SysClientVo
	ConvertToSysClientVoList(in []*systemmodel.SysClient) []*SysClientVo

	//goverter:autoMap BaseEntity
	ConvertToSysConfigVo(c *systemmodel.SysConfig) *SysConfigVo
	ConvertToSysConfigVoList(in []*systemmodel.SysConfig) []*SysConfigVo

	//goverter:autoMap BaseEntity
	//goverter:ignore Children
	ConvertToSysDeptVo(d *systemmodel.SysDept) *SysDeptVo
	ConvertToSysDeptVoList(in []*systemmodel.SysDept) []*SysDeptVo

	//goverter:autoMap BaseEntity
	ConvertToSysDictDataVo(d *systemmodel.SysDictData) *SysDictDataVo
	ConvertToSysDictDataVoList(in []*systemmodel.SysDictData) []*SysDictDataVo

	//goverter:autoMap BaseEntity
	ConvertToSysDictTypeVo(t *systemmodel.SysDictType) *SysDictTypeVo
	ConvertToSysDictTypeVoList(in []*systemmodel.SysDictType) []*SysDictTypeVo

	ConvertToSysLoginInfoVo(l *systemmodel.SysLoginInfo) *SysLoginInfoVo
	ConvertToSysLoginInfoVoList(in []*systemmodel.SysLoginInfo) []*SysLoginInfoVo

	//goverter:autoMap BaseEntity
	//goverter:ignore Children
	ConvertToSysMenuVo(m *systemmodel.SysMenu) *SysMenuVo
	ConvertToSysMenuVoList(in []*systemmodel.SysMenu) []*SysMenuVo

	//goverter:autoMap BaseEntity
	ConvertToSysMessageVo(m *systemmodel.SysMessage) *SysMessageVo
	ConvertToSysMessageVoList(in []*systemmodel.SysMessage) []*SysMessageVo

	//goverter:autoMap BaseEntity
	ConvertToSysNoticeVo(n *systemmodel.SysNotice) *SysNoticeVo
	ConvertToSysNoticeVoList(in []*systemmodel.SysNotice) []*SysNoticeVo

	ConvertToSysOperLogVo(o *systemmodel.SysOperLog) *SysOperLogVo
	ConvertToSysOperLogVoList(in []*systemmodel.SysOperLog) []*SysOperLogVo

	//goverter:autoMap BaseEntity
	ConvertToSysOssVo(o *systemmodel.SysOss) *SysOssVo
	ConvertToSysOssVoList(in []*systemmodel.SysOss) []*SysOssVo

	ConvertToSysOssConfigVo(c *systemmodel.SysOssConfig) *SysOssConfigVo
	ConvertToSysOssConfigVoList(in []*systemmodel.SysOssConfig) []*SysOssConfigVo

	//goverter:autoMap BaseEntity
	ConvertToSysPostVo(p *systemmodel.SysPost) *SysPostVo
	ConvertToSysPostVoList(in []*systemmodel.SysPost) []*SysPostVo

	//goverter:autoMap BaseEntity
	ConvertToSysRoleVo(r *systemmodel.SysRole) *SysRoleVo
	ConvertToSysRoleVoList(in []*systemmodel.SysRole) []*SysRoleVo

	//goverter:autoMap BaseEntity
	ConvertToSysSocialVo(s *systemmodel.SysSocial) *SysSocialVo
	ConvertToSysSocialVoList(in []*systemmodel.SysSocial) []*SysSocialVo

	//goverter:autoMap BaseEntity
	ConvertToSysUserVo(u *systemmodel.SysUser) *SysUserVo
	ConvertToSysUserVoList(in []*systemmodel.SysUser) []*SysUserVo

	ConvertToSysUserExportVo(u *systemmodel.SysUser) *SysUserExportVo
	ConvertToSysUserExportVoList(in []*systemmodel.SysUser) []*SysUserExportVo
}
