package bo

// SysClientStatusBo 客户端启停状态入参。
//
// 与 SysClientBo 分开而非复用：Go 的 binding tag 无分组概念，复用 SysClientBo 会让
// {clientId,status} 这样的载荷被 required 直接打回。字段只取定位键与目标状态。
type SysClientStatusBo struct {
	// ClientID 客户端标识，changeStatus 按它定位。
	ClientID string `json:"clientId" binding:"required"`
	// Status 状态（0正常 1停用）。
	Status string `json:"status" binding:"required,oneof=0 1"`
}
