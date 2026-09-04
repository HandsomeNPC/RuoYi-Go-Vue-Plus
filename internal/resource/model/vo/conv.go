package vo

import (
	resourcemodel "ruoyi-go-vue-plus/internal/resource/model"
)

// goverter 生成 entity→VO 转换器，替代手写逐字段拷贝。
// 生成：go generate ./internal/resource/model/...
//
//   - nil→nil：Pointer 构建器默认守卫，无需设置。
//   - ignoreMissing yes：无源对应字段（CreateByName 等回填字段）留零。
//   - skipCopySameType yes：*time.Time 等同类型直接拷贝，免深度转换报错。
//   - autoMap BaseEntity：提升嵌入式审计字段，使 VO 的 CreateTime/CreateBy 按名映射。
//
// SysOssConfigVo 无 autoMap：它不含任何审计字段。
//
// 命名 ConvertTo<Target>；列表加 List。
//
//go:generate go tool goverter gen .

//goverter:converter
//goverter:output:file conv_gen.go
//goverter:ignoreMissing yes
//goverter:skipCopySameType yes
type Converter interface {
	//goverter:autoMap BaseEntity
	ConvertToSysOssVo(o *resourcemodel.SysOss) *SysOssVo
	ConvertToSysOssVoList(in []*resourcemodel.SysOss) []*SysOssVo

	ConvertToSysOssConfigVo(c *resourcemodel.SysOssConfig) *SysOssConfigVo
	ConvertToSysOssConfigVoList(in []*resourcemodel.SysOssConfig) []*SysOssConfigVo
}
