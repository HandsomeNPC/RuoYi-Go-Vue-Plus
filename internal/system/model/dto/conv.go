package dto

import (
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
)

// goverter 生成 VO→DTO 转换器，替代手写逐字段拷贝。
// 对应 Java BeanUtil.copyToList(roles, RoleDTO.class) / copyToList(posts, PostDTO.class)。
// 生成：go generate ./internal/system/model/...
//
//   - nil→nil：Pointer 构建器默认守卫，无需设置。
//   - ignoreMissing yes：VO 上的回填字段/审计字段在 DTO 无对应时留零。
//   - skipCopySameType yes：同类型直接拷贝。
//
// 命名 ConvertTo<Target>；列表加 List，复用单项指针方法。
//
//go:generate go tool goverter gen .

//goverter:converter
//goverter:output:file conv_gen.go
//goverter:ignoreMissing yes
//goverter:skipCopySameType yes
type Converter interface {
	ConvertToRoleDTO(r *systemvo.SysRoleVo) *RoleDTO
	ConvertToRoleDTOList(in []*systemvo.SysRoleVo) []*RoleDTO

	ConvertToPostDTO(p *systemvo.SysPostVo) *PostDTO
	ConvertToPostDTOList(in []*systemvo.SysPostVo) []*PostDTO
}
