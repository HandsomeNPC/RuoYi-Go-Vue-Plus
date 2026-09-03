package bo

import (
	resourcemodel "ruoyi-go-vue-plus/internal/resource/model"
)

// goverter 生成 BO→entity 转换器，替代手写逐字段拷贝。
// 生成：go generate ./internal/resource/model/...
//
//   - nil→nil：Pointer 构建器默认守卫，无需设置。
//   - ignoreMissing yes：无源对应字段（BaseEntity 审计子字段等）留零。
//   - skipCopySameType yes：同类型直接拷贝，免 time.Time 未导出字段报错。
//   - map . BaseEntity：整个 BO 作为源填嵌入式 BaseEntity，按名写 CreateBy。
//
// 命名 ConvertTo<Target>；列表加 List。
//
//go:generate go tool goverter gen .

//goverter:converter
//goverter:output:file conv_gen.go
//goverter:ignoreMissing yes
//goverter:skipCopySameType yes
type Converter interface {
	//goverter:map . BaseEntity
	ConvertToSysOss(b *SysOssBo) *resourcemodel.SysOss
	ConvertToSysOssList(in []*SysOssBo) []*resourcemodel.SysOss

	ConvertToSysOssConfig(b *SysOssConfigBo) *resourcemodel.SysOssConfig
	ConvertToSysOssConfigList(in []*SysOssConfigBo) []*resourcemodel.SysOssConfig
}
