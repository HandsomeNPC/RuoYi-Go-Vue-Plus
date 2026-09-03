package bo

// SysOssConfigQueryBo 对象存储配置列表查询条件（query 参数）。
//
// 与 SysOssConfigBo 分开而非复用：后者一串 binding:"required" 是新增场景的契约，
// 查询条件全部可选。
type SysOssConfigQueryBo struct {
	ConfigKey  string `form:"configKey"`
	BucketName string `form:"bucketName"`
	Status     string `form:"status"`
}
