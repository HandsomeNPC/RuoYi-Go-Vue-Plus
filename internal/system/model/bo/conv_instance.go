//go:build !goverter

// Conv 是 goverter 生成的转换器实例，示例：bo.Conv.ConvertToSysClient(b)。
// !goverter 守卫：goverter 加载本包时排除本文件，避免引用尚未生成的 ConverterImpl。
package bo

var Conv = ConverterImpl{}
