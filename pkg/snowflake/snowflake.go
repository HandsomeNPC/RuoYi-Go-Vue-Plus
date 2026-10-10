// Package snowflake 雪花算法 ID 生成器。
//
// 各业务表主键为 bigint 且无 auto_increment，由应用层发号；GORM 无等价机制，
// repository 插入前显式调用 Next()。
//
// 64 位布局（同 Twitter 原版）：
//
//	1 符号位(恒 0) | 41 位毫秒 | 5 数据中心号 | 5 工作机器号 | 12 序列号
//
// 同毫秒最多 4096 个号，超出则自旋等下一毫秒。多进程部署每个进程必须配不同
// workerId，否则撞号，见 configs/<module>.yaml。
//
// 配置由外部注入（Init 接收 SnowflakeConfig），本包不依赖 pkg/config。
package snowflake

import (
	"fmt"
	"log"
	"sync/atomic"
)

// instance 包级默认实例。启动时 Init 写一次，之后只读，故用 atomic.Pointer 无锁读取。
var instance atomic.Pointer[Generator]

// Init 按 cfg 构造生成器并设为包级实例。配置非法时 panic。
func Init(cfg SnowflakeConfig) {
	g, err := NewGenerator(cfg)
	if err != nil {
		panic(fmt.Errorf("snowflake: 初始化失败: %w", err))
	}
	instance.Store(g)
	log.Printf("snowflake 已就绪: workerId=%d datacenterId=%d", cfg.WorkerID, cfg.DatacenterID)
}

// Next 返回包级实例发的下一个 ID，未调用 Init 会 panic。
func Next() int64 {
	g := instance.Load()
	if g == nil {
		panic("snowflake: 尚未初始化，请先调用 snowflake.Init")
	}
	return g.Next()
}
