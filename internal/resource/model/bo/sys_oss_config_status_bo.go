package bo

// SysOssConfigStatusBo 切换默认配置的入参。
//
// 另开一型而非复用 SysOssConfigBo：Java 侧 changeStatus 复用写入 BO 且不加校验组，
// 而前端只回传这三个字段，套上写入 BO 的 binding:"required" 会直接卡死该接口。
type SysOssConfigStatusBo struct {
	OssConfigID int64 `json:"ossConfigId" binding:"required"`
	// ConfigKey 仅用于切换成功后写默认配置键，不参与更新列。
	ConfigKey string `json:"configKey" binding:"required"`
	// Status 目标状态（Y是 N否）。
	Status string `json:"status" binding:"required"`
}
