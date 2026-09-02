// Package vo 缓存监控出参。
package vo

// CacheListInfoVo 缓存监控概览，对照 Java CacheController.CacheListInfoVo。
//
// Info 是 Redis INFO 全量键值；CommandStats 是命令调用统计饼图数据，
// 每项 {name,value} 形如 {"get","37"}。
type CacheListInfoVo struct {
	Info         map[string]string   `json:"info"`
	DBSize       int64               `json:"dbSize"`
	CommandStats []map[string]string `json:"commandStats"`
}
